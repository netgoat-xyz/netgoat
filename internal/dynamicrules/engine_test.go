package dynamicrules

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func newTestEngine(t *testing.T, limits Limits) *Engine {
	t.Helper()
	engine, err := NewEngine(limits)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return engine
}

func TestTypeScriptRuleReceivesFrozenDeterministicRequest(t *testing.T) {
	engine := newTestEngine(t, Limits{})
	err := engine.Reload([]Rule{{
		Name: "typescript request",
		Source: `
			type Request = {
				method: string;
				path: string;
				headers: Record<string, readonly string[]>;
				query: Record<string, readonly string[]>;
			};
			export function evaluate(request: Request) {
				if (!Object.isFrozen(request) || !Object.isFrozen(request.headers) || !Object.isFrozen(request.headers["x-test"])) {
					return { action: "block", reason: "request was mutable" };
				}
				try { (request.headers["x-test"] as string[])[0] = "changed"; } catch (_) {}
				const actual = [
					request.method,
					request.path,
					request.headers["x-test"][0],
					Object.keys(request.headers).join(","),
					Object.keys(request.query).join(","),
				].join("|");
				return actual === "GET|/admin|first|x-test,z-last|a,b"
					? { action: "block", reason: actual }
					: null;
			}
		`,
	}})
	if err != nil {
		t.Fatalf("Reload TypeScript rule: %v", err)
	}

	decision, err := engine.Evaluate(context.Background(), Request{
		Method: "GET",
		Path:   "/admin",
		Headers: map[string][]string{
			"Z-Last": {"last"},
			"X-Test": {"first"},
		},
		Query: map[string][]string{
			"b": {"2"},
			"a": {"1"},
		},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if want := (Decision{Action: ActionBlock, Reason: "GET|/admin|first|x-test,z-last|a,b", Rule: "typescript request"}); decision != want {
		t.Fatalf("decision = %#v, want %#v", decision, want)
	}
}

func TestReloadRetainsLastKnownGoodRules(t *testing.T) {
	engine := newTestEngine(t, Limits{})
	good := []Rule{{
		Name:   "block protected",
		Source: `export function evaluate(request) { return request.path === "/protected" ? {action: "block", reason: "protected"} : null; }`,
	}}
	if err := engine.Reload(good); err != nil {
		t.Fatalf("load good rules: %v", err)
	}

	before, err := engine.Evaluate(context.Background(), Request{Method: "GET", Path: "/protected"})
	if err != nil || before.Action != ActionBlock || before.Rule != "block protected" {
		t.Fatalf("before failed reload = %#v, %v", before, err)
	}

	if err := engine.Reload([]Rule{{Name: "missing handler", Source: `export const notAHandler = true;`}}); err == nil {
		t.Fatal("Reload accepted a rule without an exported handler")
	}
	after, err := engine.Evaluate(context.Background(), Request{Method: "GET", Path: "/protected"})
	if err != nil || after != before {
		t.Fatalf("failed reload changed live rules: %#v, %v; want %#v", after, err, before)
	}
}

func TestRulesHaveNoHostBindings(t *testing.T) {
	engine := newTestEngine(t, Limits{})
	if err := engine.Reload([]Rule{{
		Name: "no host bindings",
		Source: `
			export function evaluate() {
				return {
					action: typeof require === "undefined" && typeof process === "undefined" && typeof fetch === "undefined"
						? "block" : "allow",
					reason: "capability check"
				};
			}
		`,
	}}); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	decision, err := engine.Evaluate(context.Background(), Request{Method: "GET"})
	if err != nil || decision.Action != ActionBlock {
		t.Fatalf("sandbox decision = %#v, %v", decision, err)
	}
}

func TestLimitsAndInvalidResultFailClosed(t *testing.T) {
	t.Run("source", func(t *testing.T) {
		engine := newTestEngine(t, Limits{MaxSourceBytes: 16})
		err := engine.Reload([]Rule{{Name: "too large", Source: `export function evaluate() { return null; }`}})
		if !errors.Is(err, ErrLimitExceeded) {
			t.Fatalf("Reload error = %v, want ErrLimitExceeded", err)
		}
	})

	t.Run("compiled", func(t *testing.T) {
		engine := newTestEngine(t, Limits{MaxCompiledBytes: 64})
		err := engine.Reload([]Rule{{Name: "expanded", Source: `export function evaluate() { return null; }`}})
		if !errors.Is(err, ErrLimitExceeded) {
			t.Fatalf("Reload error = %v, want ErrLimitExceeded", err)
		}
	})

	t.Run("input", func(t *testing.T) {
		engine := newTestEngine(t, Limits{MaxInputBytes: 96})
		decision, err := engine.Evaluate(context.Background(), Request{Method: "GET", Body: strings.Repeat("x", 128)})
		if !errors.Is(err, ErrLimitExceeded) || decision.Action != ActionBlock {
			t.Fatalf("Evaluate = %#v, %v; want fail-closed input limit", decision, err)
		}
	})

	t.Run("result", func(t *testing.T) {
		engine := newTestEngine(t, Limits{MaxResultBytes: 96})
		if err := engine.Reload([]Rule{{Name: "large result", Source: `export function evaluate() { return {action: "block", reason: "` + strings.Repeat("x", 128) + `"}; }`}}); err != nil {
			t.Fatalf("Reload: %v", err)
		}
		decision, err := engine.Evaluate(context.Background(), Request{Method: "GET"})
		if !errors.Is(err, ErrLimitExceeded) || decision.Action != ActionBlock {
			t.Fatalf("Evaluate = %#v, %v; want fail-closed result limit", decision, err)
		}
	})

	t.Run("schema", func(t *testing.T) {
		engine := newTestEngine(t, Limits{})
		if err := engine.Reload([]Rule{{Name: "bad result", Source: `export function evaluate() { return {action: "log"}; }`}}); err != nil {
			t.Fatalf("Reload: %v", err)
		}
		decision, err := engine.Evaluate(context.Background(), Request{Method: "GET"})
		if !errors.Is(err, ErrInvalidDecision) || decision.Action != ActionBlock {
			t.Fatalf("Evaluate = %#v, %v; want fail-closed invalid schema", decision, err)
		}
	})
}

func TestExecutionDeadlineInterruptsInfiniteLoop(t *testing.T) {
	engine := newTestEngine(t, Limits{MaxExecutionDuration: 15 * time.Millisecond})
	if err := engine.Reload([]Rule{{Name: "loop", Source: `export function evaluate() { for (;;) {} }`}}); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	decision, err := engine.Evaluate(context.Background(), Request{Method: "GET"})
	if !errors.Is(err, ErrExecutionTimeout) || decision.Action != ActionBlock || decision.Rule != "loop" {
		t.Fatalf("Evaluate = %#v, %v; want timed-out block", decision, err)
	}
}

func TestContextDeadlineInterruptsInfiniteLoop(t *testing.T) {
	engine := newTestEngine(t, Limits{MaxExecutionDuration: 500 * time.Millisecond})
	if err := engine.Reload([]Rule{{Name: "loop", Source: `export default function() { while (true) {} }`}}); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	decision, err := engine.Evaluate(ctx, Request{Method: "GET"})
	if !errors.Is(err, context.DeadlineExceeded) || decision.Action != ActionBlock || decision.Rule != "loop" {
		t.Fatalf("Evaluate = %#v, %v; want context-deadline block", decision, err)
	}
}

func TestNoDecisionDefaultsToAllow(t *testing.T) {
	engine := newTestEngine(t, Limits{})
	if err := engine.Reload([]Rule{{Name: "observe", Source: `export function evaluate() { return null; }`}}); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	decision, err := engine.Evaluate(context.Background(), Request{Method: "GET"})
	if err != nil || decision != (Decision{Action: ActionAllow}) {
		t.Fatalf("Evaluate = %#v, %v", decision, err)
	}
}
