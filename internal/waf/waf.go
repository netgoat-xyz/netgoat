package waf

import (
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/rs/zerolog/log"
)

// WAFContext defines the variables exposed to the rule engine.
type WAFContext struct {
	IP       string
	Host     string
	Method   string
	Path     string
	Query    map[string][]string
	RawQuery string
	Headers  map[string][]string
}

type compiledRule struct {
	name    string
	action  string
	program *vm.Program
}

type compiledRules struct {
	items []compiledRule
}

// Engine evaluates an immutable, precompiled rule set. Reload builds a full
// replacement before publishing it, so requests never observe partial updates.
type Engine struct {
	rules atomic.Pointer[compiledRules]
}

func NewEngine() *Engine {
	engine := &Engine{}
	engine.rules.Store(&compiledRules{})
	return engine
}

// Reload compiles all database rules and atomically swaps them into service.
// The previous rule set remains active if any rule cannot be loaded.
func (e *Engine) Reload(db *sql.DB) error {
	rows, err := db.Query("SELECT name, expression, action FROM waf_rules ORDER BY priority DESC, id ASC")
	if err != nil {
		return err
	}
	defer rows.Close()

	next := &compiledRules{}
	for rows.Next() {
		var name, expression, action string
		if err := rows.Scan(&name, &expression, &action); err != nil {
			return err
		}
		program, err := compileExpression(expression)
		if err != nil {
			return fmt.Errorf("compile WAF rule %q: %w", name, err)
		}
		next.items = append(next.items, compiledRule{
			name:    name,
			action:  strings.ToUpper(strings.TrimSpace(action)),
			program: program,
		})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	e.rules.Store(next)
	return nil
}

// ValidateExpression verifies that a rule is a boolean WAF expression.
func ValidateExpression(expression string) error {
	_, err := compileExpression(expression)
	return err
}

func compileExpression(expression string) (*vm.Program, error) {
	return expr.Compile(expression, expr.Env(WAFContext{}), expr.AsBool())
}

// Check evaluates the current precompiled rule set for a request.
func (e *Engine) Check(r *http.Request, debugLogs bool) (bool, string) {
	if e == nil || r == nil {
		return false, ""
	}
	decodedQuery, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		log.Warn().Err(err).Msg("Blocked request due to malformed URL encoding")
		return true, "Block Malformed Encoding"
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	env := WAFContext{
		IP:       ip,
		Host:     normalizedHost(r.Host),
		Method:   r.Method,
		Path:     r.URL.Path,
		Query:    r.URL.Query(),
		RawQuery: decodedQuery,
		Headers:  r.Header,
	}

	rules := e.rules.Load()
	if rules == nil {
		return false, ""
	}
	for _, rule := range rules.items {
		output, err := expr.Run(rule.program, env)
		if err != nil {
			log.Error().Err(err).Str("rule", rule.name).Msg("Error running WAF rule")
			continue
		}
		matched, _ := output.(bool)
		if debugLogs {
			log.Debug().Str("rule", rule.name).Bool("matched", matched).Msg("WAF rule evaluation")
		}
		if !matched {
			continue
		}
		switch rule.action {
		case "ALLOW":
			return false, rule.name
		case "", "BLOCK":
			return true, rule.name
		}
	}
	return false, ""
}

func normalizedHost(hostport string) string {
	host := strings.TrimSpace(hostport)
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	} else {
		host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	}
	return strings.ToLower(strings.TrimSuffix(host, "."))
}

// Check is retained for callers that have not yet adopted a long-lived Engine.
func Check(db *sql.DB, r *http.Request, debugLogs bool) (bool, string) {
	engine := NewEngine()
	if err := engine.Reload(db); err != nil {
		log.Error().Err(err).Msg("Failed to load WAF rules")
		return false, ""
	}
	return engine.Check(r, debugLogs)
}
