package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"netgoat.xyz/agent/internal/cloudflare"
	"netgoat.xyz/agent/internal/config"
)

const (
	cloudflareAPITokenEnvironment           = "CLOUDFLARE_API_TOKEN"
	cloudflareReconciliationDeadline        = 2 * time.Minute
	maxCloudflareReconciliationOperations   = 32
	maxCloudflareReconciliationPayloadBytes = 64 << 10
	maxCloudflareAccessJWKSCacheSeconds     = 24 * 60 * 60
	maxCloudflareAccessClockSkewSeconds     = 5 * 60
	maxCloudflareAccessFetchTimeoutSeconds  = 30
	maxCloudflareAPIRequestTimeoutSeconds   = 60
)

type cloudflareReconciliationClient interface {
	ReconcileDNSRecord(context.Context, string, string, any) (cloudflare.APIResult, error)
	DeleteDNSRecord(context.Context, string, string) (cloudflare.APIResult, error)
	CreateTunnel(context.Context, any) (cloudflare.APIResult, error)
	ReconcileTunnel(context.Context, string, any) (cloudflare.APIResult, error)
	DeleteTunnel(context.Context, string) (cloudflare.APIResult, error)
}

var newCloudflareAPIClient = func(settings cloudflare.APIConfig) (cloudflareReconciliationClient, error) {
	return cloudflare.NewAPIClient(settings)
}

// configureCloudflareAccess constructs the request verifier without fetching
// keys. The validator fetches and caches JWKS material only when an assertion
// arrives, and its middleware rejects that request if verification fails.
func configureCloudflareAccess(cfg *config.Config) (*cloudflare.AccessValidator, error) {
	settings, err := cloudflareAccessSettingsFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return nil, nil
	}
	validator, err := cloudflare.NewAccessValidator(settings)
	if err != nil {
		return nil, fmt.Errorf("configure Cloudflare Access: %w", err)
	}
	return validator, nil
}

func cloudflareAccessSettingsFromConfig(cfg *config.Config) (cloudflare.AccessConfig, error) {
	if cfg == nil || !cfg.Cloudflare.Access.Enabled {
		return cloudflare.AccessConfig{}, nil
	}

	access := cfg.Cloudflare.Access
	cacheTTL, err := cloudflareDurationFromSeconds("cloudflare.access.jwks_cache_seconds", access.JWKSCacheSeconds, maxCloudflareAccessJWKSCacheSeconds)
	if err != nil {
		return cloudflare.AccessConfig{}, err
	}
	clockSkew, err := cloudflareDurationFromSeconds("cloudflare.access.clock_skew_seconds", access.ClockSkewSeconds, maxCloudflareAccessClockSkewSeconds)
	if err != nil {
		return cloudflare.AccessConfig{}, err
	}
	fetchTimeout, err := cloudflareDurationFromSeconds("cloudflare.access.fetch_timeout_seconds", access.FetchTimeoutSeconds, maxCloudflareAccessFetchTimeoutSeconds)
	if err != nil {
		return cloudflare.AccessConfig{}, err
	}

	settings := cloudflare.AccessConfig{
		Enabled:      true,
		Issuer:       access.Issuer,
		Audience:     append([]string(nil), access.Audience...),
		JWKSURL:      access.JWKSURL,
		Header:       access.Header,
		Cookie:       access.Cookie,
		CacheTTL:     cacheTTL,
		ClockSkew:    clockSkew,
		FetchTimeout: fetchTimeout,
	}
	if err := settings.Validate(); err != nil {
		return cloudflare.AccessConfig{}, fmt.Errorf("invalid cloudflare.access configuration: %w", err)
	}
	return settings, nil
}

type cloudflareReconciliationPlan struct {
	apiConfig cloudflare.APIConfig
	dns       []cloudflareDNSOperation
	tunnels   []cloudflareTunnelOperation
}

type cloudflareDNSOperation struct {
	zoneID   string
	recordID string
	delete   bool
	desired  map[string]any
}

type cloudflareTunnelOperation struct {
	tunnelID string
	delete   bool
	desired  map[string]any
}

// cloudflareReconciliationPlanFromConfig validates a finite, explicit startup
// plan before any API request is made. Its default dry-run behavior ensures a
// config review cannot modify Cloudflare until dry_run is explicitly false.
func cloudflareReconciliationPlanFromConfig(cfg *config.Config) (*cloudflareReconciliationPlan, error) {
	if cfg == nil || !cfg.Cloudflare.Reconciliation.Enabled {
		return nil, nil
	}

	reconciliation := cfg.Cloudflare.Reconciliation
	operations := len(reconciliation.DNSRecords) + len(reconciliation.Tunnels)
	if operations == 0 {
		return nil, errors.New("cloudflare.reconciliation.enabled requires at least one DNS record or tunnel")
	}
	if operations > maxCloudflareReconciliationOperations {
		return nil, fmt.Errorf("cloudflare reconciliation has %d operations; maximum is %d", operations, maxCloudflareReconciliationOperations)
	}

	requestTimeout, err := cloudflareDurationFromSeconds(
		"cloudflare.reconciliation.request_timeout_seconds",
		reconciliation.RequestTimeoutSeconds,
		maxCloudflareAPIRequestTimeoutSeconds,
	)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(os.Getenv(cloudflareAPITokenEnvironment))
	if token == "" {
		return nil, fmt.Errorf("%s must be set when cloudflare.reconciliation is enabled", cloudflareAPITokenEnvironment)
	}
	dryRun := true
	if reconciliation.DryRun != nil {
		dryRun = *reconciliation.DryRun
	}
	apiConfig := cloudflare.APIConfig{
		Enabled:        true,
		APIToken:       token,
		AccountID:      strings.TrimSpace(reconciliation.AccountID),
		DryRun:         dryRun,
		RequestTimeout: requestTimeout,
	}
	if err := apiConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid cloudflare.reconciliation configuration: %w", err)
	}

	plan := &cloudflareReconciliationPlan{
		apiConfig: apiConfig,
		dns:       make([]cloudflareDNSOperation, 0, len(reconciliation.DNSRecords)),
		tunnels:   make([]cloudflareTunnelOperation, 0, len(reconciliation.Tunnels)),
	}
	for index, record := range reconciliation.DNSRecords {
		zoneID := strings.TrimSpace(record.ZoneID)
		recordID := strings.TrimSpace(record.RecordID)
		if zoneID == "" {
			return nil, fmt.Errorf("cloudflare DNS record %d is missing zone_id", index+1)
		}
		if record.Delete {
			if recordID == "" {
				return nil, fmt.Errorf("cloudflare DNS record %d deletion requires record_id", index+1)
			}
			if len(record.Record) != 0 {
				return nil, fmt.Errorf("cloudflare DNS record %d cannot set record when delete is true", index+1)
			}
			plan.dns = append(plan.dns, cloudflareDNSOperation{zoneID: zoneID, recordID: recordID, delete: true})
			continue
		}
		desired, err := cloudflareDesiredObject(fmt.Sprintf("cloudflare DNS record %d", index+1), record.Record)
		if err != nil {
			return nil, err
		}
		plan.dns = append(plan.dns, cloudflareDNSOperation{zoneID: zoneID, recordID: recordID, desired: desired})
	}

	if len(reconciliation.Tunnels) > 0 && apiConfig.AccountID == "" {
		return nil, errors.New("cloudflare.reconciliation.account_id is required when tunnels are configured")
	}
	for index, tunnel := range reconciliation.Tunnels {
		tunnelID := strings.TrimSpace(tunnel.TunnelID)
		if tunnel.Delete {
			if tunnelID == "" {
				return nil, fmt.Errorf("cloudflare tunnel %d deletion requires tunnel_id", index+1)
			}
			if len(tunnel.Tunnel) != 0 {
				return nil, fmt.Errorf("cloudflare tunnel %d cannot set tunnel when delete is true", index+1)
			}
			plan.tunnels = append(plan.tunnels, cloudflareTunnelOperation{tunnelID: tunnelID, delete: true})
			continue
		}
		desired, err := cloudflareDesiredObject(fmt.Sprintf("cloudflare tunnel %d", index+1), tunnel.Tunnel)
		if err != nil {
			return nil, err
		}
		plan.tunnels = append(plan.tunnels, cloudflareTunnelOperation{tunnelID: tunnelID, desired: desired})
	}
	if err := validateCloudflareReconciliationPlan(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func cloudflareDesiredObject(name string, source map[string]any) (map[string]any, error) {
	if len(source) == 0 {
		return nil, fmt.Errorf("%s requires a non-empty declarative payload", name)
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("%s payload is not valid JSON: %w", name, err)
	}
	if len(encoded) > maxCloudflareReconciliationPayloadBytes {
		return nil, fmt.Errorf("%s payload exceeds %d bytes", name, maxCloudflareReconciliationPayloadBytes)
	}
	var copied map[string]any
	if err := json.Unmarshal(encoded, &copied); err != nil || len(copied) == 0 {
		return nil, fmt.Errorf("%s payload is not a JSON object", name)
	}
	return copied, nil
}

// validateCloudflareReconciliationPlan exercises the API client's identifier
// and request-payload validation against a forced dry-run client. It performs
// no network I/O, but catches malformed IDs before an earlier non-dry-run
// operation could be applied.
func validateCloudflareReconciliationPlan(plan *cloudflareReconciliationPlan) error {
	if plan == nil {
		return nil
	}
	settings := plan.apiConfig
	settings.DryRun = true
	client, err := cloudflare.NewAPIClient(settings)
	if err != nil {
		return fmt.Errorf("validate Cloudflare reconciliation client: %w", err)
	}
	for index, operation := range plan.dns {
		if operation.delete {
			_, err = client.DeleteDNSRecord(context.Background(), operation.zoneID, operation.recordID)
		} else {
			_, err = client.ReconcileDNSRecord(context.Background(), operation.zoneID, operation.recordID, operation.desired)
		}
		if err != nil {
			return fmt.Errorf("invalid Cloudflare DNS record %d: %w", index+1, err)
		}
	}
	for index, operation := range plan.tunnels {
		if operation.delete {
			_, err = client.DeleteTunnel(context.Background(), operation.tunnelID)
		} else if operation.tunnelID == "" {
			_, err = client.CreateTunnel(context.Background(), operation.desired)
		} else {
			_, err = client.ReconcileTunnel(context.Background(), operation.tunnelID, operation.desired)
		}
		if err != nil {
			return fmt.Errorf("invalid Cloudflare tunnel %d: %w", index+1, err)
		}
	}
	return nil
}

// reconcileCloudflare applies the already bounded plan exactly once. It does
// not schedule retries or make background changes; a restart is required for
// another reconciliation run.
func reconcileCloudflare(ctx context.Context, cfg *config.Config) ([]cloudflare.APIResult, error) {
	plan, err := cloudflareReconciliationPlanFromConfig(cfg)
	if err != nil || plan == nil {
		return nil, err
	}
	client, err := newCloudflareAPIClient(plan.apiConfig)
	if err != nil {
		return nil, fmt.Errorf("create Cloudflare API client: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	results := make([]cloudflare.APIResult, 0, len(plan.dns)+len(plan.tunnels))
	for index, operation := range plan.dns {
		var result cloudflare.APIResult
		if operation.delete {
			result, err = client.DeleteDNSRecord(ctx, operation.zoneID, operation.recordID)
		} else {
			result, err = client.ReconcileDNSRecord(ctx, operation.zoneID, operation.recordID, operation.desired)
		}
		if err != nil {
			return results, fmt.Errorf("reconcile Cloudflare DNS record %d: %w", index+1, err)
		}
		results = append(results, result)
	}
	for index, operation := range plan.tunnels {
		var result cloudflare.APIResult
		if operation.delete {
			result, err = client.DeleteTunnel(ctx, operation.tunnelID)
		} else if operation.tunnelID == "" {
			result, err = client.CreateTunnel(ctx, operation.desired)
		} else {
			result, err = client.ReconcileTunnel(ctx, operation.tunnelID, operation.desired)
		}
		if err != nil {
			return results, fmt.Errorf("reconcile Cloudflare tunnel %d: %w", index+1, err)
		}
		results = append(results, result)
	}
	return results, nil
}

func cloudflareDurationFromSeconds(name string, seconds, maximum int) (time.Duration, error) {
	if seconds < 0 || seconds > maximum {
		return 0, fmt.Errorf("%s must be between zero and %d seconds", name, maximum)
	}
	return time.Duration(seconds) * time.Second, nil
}
