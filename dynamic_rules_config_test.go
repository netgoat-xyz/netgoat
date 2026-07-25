package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"netgoat.xyz/agent/internal/config"
	"netgoat.xyz/agent/internal/dynamicrules"
)

func TestConfigureDynamicRulesEvaluatesAndRestoresRequestBody(t *testing.T) {
	cfg := &config.Config{}
	cfg.DynamicRules.Enabled = true
	cfg.DynamicRules.MaxInputBytes = 512
	cfg.DynamicRules.Rules = []config.DynamicRule{{
		Name:     "block-secret",
		Language: "typescript",
		Source: `export function evaluate(request) {
			return request.body === "secret" ? { action: "block", reason: "secret body" } : null;
		}`,
	}}
	engine, err := configureDynamicRules(cfg)
	if err != nil {
		t.Fatalf("configureDynamicRules(): %v", err)
	}
	if engine == nil {
		t.Fatal("configureDynamicRules() returned nil engine")
	}

	request := httptest.NewRequest(http.MethodPost, "http://api.example.test/submit", strings.NewReader("secret"))
	decision, err := evaluateDynamicRules(request, engine, "203.0.113.7")
	if err != nil || decision.Action != dynamicrules.ActionBlock || decision.Rule != "block-secret" {
		t.Fatalf("evaluateDynamicRules() = %#v, %v", decision, err)
	}
	body, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatalf("read restored body: %v", err)
	}
	if string(body) != "secret" {
		t.Fatalf("restored body = %q", body)
	}
	retryBody, err := request.GetBody()
	if err != nil {
		t.Fatalf("GetBody(): %v", err)
	}
	defer retryBody.Close()
	if copied, _ := io.ReadAll(retryBody); string(copied) != "secret" {
		t.Fatalf("GetBody() = %q", copied)
	}
}

func TestDynamicRulesRejectInvalidConfigAndBoundBodies(t *testing.T) {
	cfg := &config.Config{}
	cfg.DynamicRules.Enabled = true
	cfg.DynamicRules.Rules = []config.DynamicRule{{Name: "broken", Source: "export function"}}
	if _, err := configureDynamicRules(cfg); err == nil {
		t.Fatal("configureDynamicRules() accepted invalid source")
	}

	cfg.DynamicRules.Rules = []config.DynamicRule{{Name: "disabled", Source: "export function evaluate() { return null; }", Enabled: boolPointer(false)}}
	if engine, err := configureDynamicRules(cfg); err != nil || engine != nil {
		t.Fatalf("disabled rules engine = %v, %v", engine, err)
	}

	request := httptest.NewRequest(http.MethodPost, "http://api.example.test/", strings.NewReader("12345"))
	if _, err := copyRequestBodyForDynamicRules(request, 4); !errors.Is(err, dynamicrules.ErrLimitExceeded) {
		t.Fatalf("copyRequestBodyForDynamicRules() error = %v, want limit error", err)
	}
}

func TestDynamicRulesRuntimeRetainsLastKnownGoodEngine(t *testing.T) {
	cfg := &config.Config{}
	cfg.DynamicRules.Enabled = true
	cfg.DynamicRules.Rules = []config.DynamicRule{{
		Name:   "stable",
		Source: `export function evaluate(request) { return request.path === "/blocked" ? { action: "block" } : null; }`,
	}}
	runtime, err := newDynamicRulesRuntime(cfg)
	if err != nil {
		t.Fatalf("newDynamicRulesRuntime(): %v", err)
	}

	cfg.DynamicRules.Rules = []config.DynamicRule{{Name: "broken", Source: "export function"}}
	if err := runtime.Update(cfg); err == nil {
		t.Fatal("Update() accepted invalid replacement rules")
	}
	request := httptest.NewRequest(http.MethodGet, "http://api.example.test/blocked", nil)
	decision, err := evaluateDynamicRules(request, runtime.Load(), "203.0.113.7")
	if err != nil || decision.Action != dynamicrules.ActionBlock || decision.Rule != "stable" {
		t.Fatalf("last-known-good evaluation = %#v, %v", decision, err)
	}
}

func boolPointer(value bool) *bool {
	return &value
}
