package dynamicrules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
)

var (
	// ErrLimitExceeded means a configured safety boundary rejected untrusted
	// source, request data, compiled output, or result data.
	ErrLimitExceeded = errors.New("dynamic rule limit exceeded")
	// ErrExecutionTimeout means the engine's per-rule execution limit expired.
	ErrExecutionTimeout = errors.New("dynamic rule execution timed out")
	// ErrInvalidDecision means a handler returned a value outside the explicit
	// {action: "allow"|"block", reason?: string} schema.
	ErrInvalidDecision = errors.New("invalid dynamic rule decision")
)

type compiledRule struct {
	name    string
	program *goja.Program
}

type compiledRules struct {
	items []compiledRule
}

// Engine holds an immutable, atomically published set of compiled rules. A
// failed reload never replaces the last-known-good set.
type Engine struct {
	limits Limits
	rules  atomic.Pointer[compiledRules]
}

// NewEngine creates an engine with validated, bounded limits.
func NewEngine(limits Limits) (*Engine, error) {
	normalized, err := normalizeLimits(limits)
	if err != nil {
		return nil, err
	}
	engine := &Engine{limits: normalized}
	engine.rules.Store(&compiledRules{})
	return engine, nil
}

// New is an alias for NewEngine.
func New(limits Limits) (*Engine, error) {
	return NewEngine(limits)
}

// Limits returns the immutable effective limits used by the engine.
func (e *Engine) Limits() Limits {
	if e == nil {
		return Limits{}
	}
	return e.limits
}

// Reload transforms, compiles, and validates an entire replacement set before
// publishing it. Rules retain the input order. A handler must export evaluate,
// a default function, or module.exports as a function.
func (e *Engine) Reload(rules []Rule) error {
	if e == nil {
		return fmt.Errorf("dynamic rules: nil engine")
	}
	if len(rules) > e.limits.MaxRules {
		return limitError("rule count", len(rules), e.limits.MaxRules)
	}

	next := &compiledRules{items: make([]compiledRule, 0, len(rules))}
	seenNames := make(map[string]struct{}, len(rules))
	for index, rule := range rules {
		compiled, err := compileRule(rule, e.limits)
		if err != nil {
			return fmt.Errorf("compile rule %d: %w", index, err)
		}
		if _, exists := seenNames[compiled.name]; exists {
			return fmt.Errorf("compile rule %d: duplicate rule name %q", index, compiled.name)
		}
		seenNames[compiled.name] = struct{}{}
		if err := validateCompiledRule(compiled, e.limits); err != nil {
			return fmt.Errorf("validate rule %q: %w", compiled.name, err)
		}
		next.items = append(next.items, compiled)
	}

	e.rules.Store(next)
	return nil
}

// Evaluate evaluates a stable snapshot of rules against a canonical request.
// A null/undefined rule result means "no decision" and evaluation continues.
// The first explicit allow or block decision wins; no decision defaults to
// allow. Every evaluation error returns a block decision as well as the error,
// so callers have a safe result even if they only retain the decision.
func (e *Engine) Evaluate(ctx context.Context, request Request) (Decision, error) {
	if e == nil {
		return blockedDecision("", fmt.Errorf("dynamic rules: nil engine"))
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return blockedDecision("", err)
	}

	input, err := normalizeRequest(request, e.limits.MaxInputBytes)
	if err != nil {
		return blockedDecision("", err)
	}
	if err := ctx.Err(); err != nil {
		return blockedDecision("", err)
	}

	current := e.rules.Load()
	if current == nil {
		return Decision{Action: ActionAllow}, nil
	}
	for _, rule := range current.items {
		decision, matched, err := executeCompiledRule(ctx, rule, input, e.limits)
		if err != nil {
			return blockedDecision(rule.name, err)
		}
		if matched {
			decision.Rule = rule.name
			if !decisionFits(decision, e.limits.MaxResultBytes) {
				return blockedDecision(rule.name, limitError("decision result", decisionJSONSize(decision), e.limits.MaxResultBytes))
			}
			return decision, nil
		}
	}
	return Decision{Action: ActionAllow}, nil
}

func blockedDecision(rule string, err error) (Decision, error) {
	return Decision{
		Action: ActionBlock,
		Reason: "rule failure",
		Rule:   rule,
	}, err
}

func normalizeLimits(limits Limits) (Limits, error) {
	defaults := DefaultLimits()
	if limits.MaxRules == 0 {
		limits.MaxRules = defaults.MaxRules
	}
	if limits.MaxSourceBytes == 0 {
		limits.MaxSourceBytes = defaults.MaxSourceBytes
	}
	if limits.MaxCompiledBytes == 0 {
		limits.MaxCompiledBytes = defaults.MaxCompiledBytes
	}
	if limits.MaxInputBytes == 0 {
		limits.MaxInputBytes = defaults.MaxInputBytes
	}
	if limits.MaxResultBytes == 0 {
		limits.MaxResultBytes = defaults.MaxResultBytes
	}
	if limits.MaxExecutionDuration == 0 {
		limits.MaxExecutionDuration = defaults.MaxExecutionDuration
	}

	for _, field := range []struct {
		name  string
		value int
		max   int
	}{
		{"max rules", limits.MaxRules, maximumRules},
		{"max source bytes", limits.MaxSourceBytes, maximumSourceBytes},
		{"max compiled bytes", limits.MaxCompiledBytes, maximumCompiledBytes},
		{"max input bytes", limits.MaxInputBytes, maximumInputBytes},
		{"max result bytes", limits.MaxResultBytes, maximumResultBytes},
	} {
		if field.value <= 0 {
			return Limits{}, fmt.Errorf("dynamic rules: %s must be positive", field.name)
		}
		if field.value > field.max {
			return Limits{}, fmt.Errorf("dynamic rules: %s exceeds hard maximum %d", field.name, field.max)
		}
	}
	if limits.MaxResultBytes < minimumResultBytes {
		return Limits{}, fmt.Errorf("dynamic rules: max result bytes must be at least %d", minimumResultBytes)
	}
	if limits.MaxExecutionDuration <= 0 {
		return Limits{}, fmt.Errorf("dynamic rules: max execution duration must be positive")
	}
	if limits.MaxExecutionDuration > maximumExecution {
		return Limits{}, fmt.Errorf("dynamic rules: max execution duration exceeds hard maximum %s", maximumExecution)
	}
	return limits, nil
}

func compileRule(rule Rule, limits Limits) (compiledRule, error) {
	name := strings.TrimSpace(rule.Name)
	if name == "" {
		return compiledRule{}, fmt.Errorf("rule name is required")
	}
	if !utf8.ValidString(name) {
		return compiledRule{}, fmt.Errorf("rule name is not valid UTF-8")
	}
	if len(name) > maximumRuleNameBytes {
		return compiledRule{}, limitError("rule name", len(name), maximumRuleNameBytes)
	}
	if !decisionFits(Decision{Action: ActionBlock, Reason: "rule failure", Rule: name}, limits.MaxResultBytes) {
		return compiledRule{}, limitError("rule name in decision", decisionJSONSize(Decision{Action: ActionBlock, Reason: "rule failure", Rule: name}), limits.MaxResultBytes)
	}
	if !utf8.ValidString(rule.Source) {
		return compiledRule{}, fmt.Errorf("rule source is not valid UTF-8")
	}
	if len(rule.Source) > limits.MaxSourceBytes {
		return compiledRule{}, limitError("rule source", len(rule.Source), limits.MaxSourceBytes)
	}

	language, err := normalizeLanguage(rule.Language)
	if err != nil {
		return compiledRule{}, err
	}
	loader := api.LoaderTS
	if language == LanguageJavaScript {
		loader = api.LoaderJS
	}
	transformed := api.Transform(rule.Source, api.TransformOptions{
		Loader:        loader,
		Format:        api.FormatCommonJS,
		Platform:      api.PlatformNeutral,
		Target:        api.ES2020,
		Sourcemap:     api.SourceMapNone,
		LegalComments: api.LegalCommentsNone,
		Sourcefile:    name,
	})
	if len(transformed.Errors) > 0 {
		return compiledRule{}, fmt.Errorf("TypeScript transform: %s", esbuildError(transformed.Errors))
	}
	if len(transformed.Code) > limits.MaxCompiledBytes {
		return compiledRule{}, limitError("compiled rule", len(transformed.Code), limits.MaxCompiledBytes)
	}
	program, err := goja.Compile(name, string(transformed.Code), true)
	if err != nil {
		return compiledRule{}, fmt.Errorf("compile JavaScript: %w", err)
	}
	return compiledRule{name: name, program: program}, nil
}

func esbuildError(messages []api.Message) string {
	if len(messages) == 0 {
		return "unknown transform error"
	}
	message := strings.TrimSpace(messages[0].Text)
	if message == "" {
		message = "unknown transform error"
	}
	const maxErrorBytes = 1024
	if len(message) > maxErrorBytes {
		return message[:maxErrorBytes] + "..."
	}
	return message
}

func validateCompiledRule(rule compiledRule, limits Limits) error {
	rt, module, err := newRuleRuntime()
	if err != nil {
		return err
	}
	stop := interruptRuntime(rt, context.Background(), limits.MaxExecutionDuration)
	defer stop()
	if _, err := rt.RunProgram(rule.program); err != nil {
		return executionError(rule.name, "initialization", err)
	}
	if _, err := handlerFromModule(rt, module); err != nil {
		return err
	}
	return nil
}

func executeCompiledRule(ctx context.Context, rule compiledRule, request normalizedRequest, limits Limits) (Decision, bool, error) {
	rt, module, err := newRuleRuntime()
	if err != nil {
		return Decision{}, false, err
	}
	requestValue, err := request.value(rt)
	if err != nil {
		return Decision{}, false, err
	}
	if err := ctx.Err(); err != nil {
		return Decision{}, false, err
	}

	stop := interruptRuntime(rt, ctx, limits.MaxExecutionDuration)
	defer stop()
	if _, err := rt.RunProgram(rule.program); err != nil {
		return Decision{}, false, executionError(rule.name, "initialization", err)
	}
	handler, err := handlerFromModule(rt, module)
	if err != nil {
		return Decision{}, false, fmt.Errorf("rule %q handler: %w", rule.name, err)
	}
	result, err := handler(goja.Undefined(), requestValue)
	if err != nil {
		return Decision{}, false, executionError(rule.name, "evaluation", err)
	}
	decision, matched, err := decodeDecision(rt, result, limits.MaxResultBytes)
	if err != nil {
		return Decision{}, false, fmt.Errorf("rule %q result: %w", rule.name, err)
	}
	return decision, matched, nil
}

func newRuleRuntime() (*goja.Runtime, *goja.Object, error) {
	rt := goja.New()
	module, err := newDataObject(rt)
	if err != nil {
		return nil, nil, err
	}
	exports, err := newDataObject(rt)
	if err != nil {
		return nil, nil, err
	}
	if err := module.DefineDataProperty("exports", exports, goja.FLAG_TRUE, goja.FLAG_TRUE, goja.FLAG_TRUE); err != nil {
		return nil, nil, fmt.Errorf("create module exports: %w", err)
	}
	if err := rt.Set("module", module); err != nil {
		return nil, nil, fmt.Errorf("install module: %w", err)
	}
	if err := rt.Set("exports", exports); err != nil {
		return nil, nil, fmt.Errorf("install exports: %w", err)
	}
	return rt, module, nil
}

func handlerFromModule(rt *goja.Runtime, module *goja.Object) (goja.Callable, error) {
	exported, err := getProperty(rt, module, "exports")
	if err != nil {
		return nil, fmt.Errorf("read module exports: %w", err)
	}
	if function, ok := goja.AssertFunction(exported); ok {
		return function, nil
	}
	if exported == nil || goja.IsUndefined(exported) || goja.IsNull(exported) {
		return nil, fmt.Errorf("missing evaluate export")
	}
	object, ok := exported.(*goja.Object)
	if !ok {
		return nil, fmt.Errorf("evaluate export is not a function")
	}
	for _, name := range []string{"evaluate", "default"} {
		value, err := getProperty(rt, object, name)
		if err != nil {
			return nil, fmt.Errorf("read %s export: %w", name, err)
		}
		if function, ok := goja.AssertFunction(value); ok {
			return function, nil
		}
	}
	return nil, fmt.Errorf("missing evaluate or default function export")
}

func getProperty(rt *goja.Runtime, object *goja.Object, name string) (value goja.Value, err error) {
	if exception := rt.Try(func() {
		value = object.Get(name)
	}); exception != nil {
		return nil, exception
	}
	return value, nil
}

func decodeDecision(rt *goja.Runtime, result goja.Value, maxBytes int) (Decision, bool, error) {
	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return Decision{}, false, nil
	}
	object, ok := result.(*goja.Object)
	if !ok || object.ClassName() != "Object" {
		return Decision{}, false, fmt.Errorf("%w: expected a plain object or null", ErrInvalidDecision)
	}

	var keys []string
	if exception := rt.Try(func() {
		keys = object.Keys()
	}); exception != nil {
		return Decision{}, false, exception
	}
	if len(keys) == 0 || len(keys) > 2 {
		return Decision{}, false, fmt.Errorf("%w: expected action and optional reason only", ErrInvalidDecision)
	}
	hasAction := false
	hasReason := false
	for _, key := range keys {
		switch key {
		case "action":
			hasAction = true
		case "reason":
			hasReason = true
		default:
			return Decision{}, false, fmt.Errorf("%w: unsupported field %q", ErrInvalidDecision, key)
		}
	}
	if !hasAction {
		return Decision{}, false, fmt.Errorf("%w: action is required", ErrInvalidDecision)
	}

	actionValue, err := getProperty(rt, object, "action")
	if err != nil {
		return Decision{}, false, err
	}
	actionString, ok := primitiveString(actionValue)
	if !ok {
		return Decision{}, false, fmt.Errorf("%w: action must be a string", ErrInvalidDecision)
	}
	decision := Decision{Action: Action(actionString)}
	if !decision.Action.Valid() {
		return Decision{}, false, fmt.Errorf("%w: unsupported action %q", ErrInvalidDecision, actionString)
	}
	if hasReason {
		reasonValue, err := getProperty(rt, object, "reason")
		if err != nil {
			return Decision{}, false, err
		}
		reason, ok := primitiveString(reasonValue)
		if !ok {
			return Decision{}, false, fmt.Errorf("%w: reason must be a string", ErrInvalidDecision)
		}
		decision.Reason = reason
	}
	if !decisionFits(decision, maxBytes) {
		return Decision{}, false, limitError("decision result", decisionJSONSize(decision), maxBytes)
	}
	return decision, true, nil
}

func primitiveString(value goja.Value) (string, bool) {
	if value == nil {
		return "", false
	}
	if _, isObject := value.(*goja.Object); isObject {
		return "", false
	}
	if value.ExportType() != reflect.TypeOf("") {
		return "", false
	}
	return value.String(), true
}

func decisionFits(decision Decision, maxBytes int) bool {
	return decisionJSONSize(decision) <= maxBytes
}

func decisionJSONSize(decision Decision) int {
	encoded, err := json.Marshal(decision)
	if err != nil {
		return maximumResultBytes + 1
	}
	return len(encoded)
}

func limitError(name string, got, max int) error {
	return fmt.Errorf("%w: %s is %d bytes/items; maximum is %d", ErrLimitExceeded, name, got, max)
}

func executionError(ruleName, phase string, err error) error {
	var interrupted *goja.InterruptedError
	if errors.As(err, &interrupted) {
		if cause, ok := interrupted.Value().(error); ok {
			return fmt.Errorf("rule %q %s interrupted: %w", ruleName, phase, cause)
		}
		return fmt.Errorf("rule %q %s: %w", ruleName, phase, ErrExecutionTimeout)
	}
	return fmt.Errorf("rule %q %s: %w", ruleName, phase, err)
}

func interruptRuntime(rt *goja.Runtime, ctx context.Context, limit time.Duration) func() {
	stopped := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		timer := time.NewTimer(limit)
		defer timer.Stop()
		select {
		case <-stopped:
			return
		case <-ctx.Done():
			rt.Interrupt(ctx.Err())
		case <-timer.C:
			rt.Interrupt(ErrExecutionTimeout)
		}
	}()
	return func() {
		close(stopped)
		wait.Wait()
	}
}
