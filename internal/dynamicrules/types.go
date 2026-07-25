package dynamicrules

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Action is the only terminal result a rule may return.
type Action string

const (
	// ActionAllow permits a request. An allow decision is terminal, allowing a
	// deliberately ordered rule set to define explicit exceptions.
	ActionAllow Action = "allow"
	// ActionBlock rejects a request.
	ActionBlock Action = "block"
)

// Valid reports whether the action is a defined terminal decision.
func (a Action) Valid() bool {
	return a == ActionAllow || a == ActionBlock
}

// Decision is the bounded, explicit result of a rule evaluation. Rule is set
// by the engine and cannot be supplied by JavaScript.
type Decision struct {
	Action Action `json:"action"`
	Reason string `json:"reason,omitempty"`
	Rule   string `json:"rule,omitempty"`
}

// Language identifies the source language accepted by esbuild. TypeScript is
// the default because it is also a superset of ordinary JavaScript syntax.
type Language string

const (
	LanguageTypeScript Language = "typescript"
	LanguageJavaScript Language = "javascript"
)

// Rule is one ordered source unit. Rules are evaluated in the order given to
// Reload; the first non-null decision is terminal.
type Rule struct {
	Name     string
	Source   string
	Language Language
}

// Request is the complete data a dynamic rule can inspect. It intentionally
// contains no live *http.Request or ResponseWriter. ClientIP must be resolved
// by a trusted proxy-aware caller before it is assigned.
//
// Headers are canonicalized to lower case when exposed to JavaScript. Query
// keys retain their case because URL query keys are case-sensitive.
type Request struct {
	Method   string
	Scheme   string
	Host     string
	Path     string
	RawQuery string
	Query    map[string][]string
	Headers  map[string][]string
	Body     string
	ClientIP string
}

// RequestFromHTTP copies a request into the limited data model. The caller
// supplies body explicitly so this helper never consumes or rewinds r.Body.
// A trusted caller may set the returned ClientIP field after resolving it.
func RequestFromHTTP(r *http.Request, body []byte) (Request, error) {
	if r == nil || r.URL == nil {
		return Request{}, fmt.Errorf("dynamic rules: nil HTTP request")
	}

	scheme := r.URL.Scheme
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := r.Host
	if host == "" {
		host = r.URL.Host
	}

	return Request{
		Method:   r.Method,
		Scheme:   scheme,
		Host:     host,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
		Query:    cloneStringValues(r.URL.Query()),
		Headers:  cloneStringValues(r.Header),
		Body:     string(body),
	}, nil
}

// Limits bounds untrusted work. A zero field receives the documented default;
// values above the package ceiling are rejected rather than silently relaxed.
type Limits struct {
	MaxRules             int
	MaxSourceBytes       int
	MaxCompiledBytes     int
	MaxInputBytes        int
	MaxResultBytes       int
	MaxExecutionDuration time.Duration
}

const (
	defaultMaxRules         = 64
	defaultMaxSourceBytes   = 64 << 10
	defaultMaxCompiledBytes = 256 << 10
	defaultMaxInputBytes    = 64 << 10
	defaultMaxResultBytes   = 4 << 10
	defaultExecutionLimit   = 25 * time.Millisecond

	maximumRules         = 256
	maximumSourceBytes   = 1 << 20
	maximumCompiledBytes = 4 << 20
	maximumInputBytes    = 1 << 20
	maximumResultBytes   = 64 << 10
	maximumExecution     = 2 * time.Second
	minimumResultBytes   = 64
	maximumRuleNameBytes = 256
)

// DefaultLimits returns the bounded defaults used for zero-valued Limits.
func DefaultLimits() Limits {
	return Limits{
		MaxRules:             defaultMaxRules,
		MaxSourceBytes:       defaultMaxSourceBytes,
		MaxCompiledBytes:     defaultMaxCompiledBytes,
		MaxInputBytes:        defaultMaxInputBytes,
		MaxResultBytes:       defaultMaxResultBytes,
		MaxExecutionDuration: defaultExecutionLimit,
	}
}

func normalizeLanguage(language Language) (Language, error) {
	switch Language(strings.ToLower(strings.TrimSpace(string(language)))) {
	case "", LanguageTypeScript, "ts":
		return LanguageTypeScript, nil
	case LanguageJavaScript, "js":
		return LanguageJavaScript, nil
	default:
		return "", fmt.Errorf("unsupported rule language %q", language)
	}
}
