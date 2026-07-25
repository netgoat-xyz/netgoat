package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	defaultAPIBaseURL     = "https://api.cloudflare.com/client/v4"
	defaultRequestTimeout = 15 * time.Second
	maxAPIBodySize        = 1 << 20
)

var (
	// ErrAPIUnavailable means the API client was not deliberately enabled or
	// has no configured API token. It is intentionally generic so callers do
	// not accidentally disclose credential state.
	ErrAPIUnavailable = errors.New("cloudflare api is unavailable")
	// ErrInvalidIdentifier means an API operation received a malformed
	// Cloudflare resource ID before it attempted a network request.
	ErrInvalidIdentifier = errors.New("invalid cloudflare identifier")
)

// APIConfig configures the opt-in Cloudflare API client. The client does not
// perform any network action unless Enabled is true and a token is supplied.
// BaseURL must be HTTPS; use a TLS test server for test injection.
type APIConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	APIToken string `json:"-" yaml:"-"`
	// AccountID is only required by tunnel operations. DNS operations use the
	// zone ID supplied to each call.
	AccountID string `json:"account_id" yaml:"account_id"`
	BaseURL   string `json:"base_url" yaml:"base_url"`
	DryRun    bool   `json:"dry_run" yaml:"dry_run"`

	RequestTimeout time.Duration `json:"request_timeout" yaml:"request_timeout"`
	HTTPClient     *http.Client  `json:"-" yaml:"-"`
}

// Validate checks a Cloudflare API configuration. A disabled client may omit
// its token, which makes it safe to keep API configuration in a shared config
// file until a deployment explicitly enables it.
func (config APIConfig) Validate() error {
	_, err := config.normalized()
	return err
}

func (config APIConfig) normalized() (APIConfig, error) {
	config.BaseURL = strings.TrimSpace(config.BaseURL)
	if config.BaseURL == "" {
		config.BaseURL = defaultAPIBaseURL
	}
	if err := validateHTTPSURL("Cloudflare API base URL", config.BaseURL); err != nil {
		return APIConfig{}, err
	}

	config.AccountID = strings.TrimSpace(config.AccountID)
	if config.AccountID != "" && !validCloudflareHexID(config.AccountID) {
		return APIConfig{}, fmt.Errorf("%w: account ID", ErrInvalidIdentifier)
	}
	if config.Enabled {
		if err := validateAPIToken(config.APIToken); err != nil {
			return APIConfig{}, err
		}
	}

	if config.RequestTimeout < 0 || config.RequestTimeout > time.Minute {
		return APIConfig{}, errors.New("Cloudflare API request timeout must be between zero and one minute")
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = defaultRequestTimeout
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: config.RequestTimeout}
	}
	return config, nil
}

func validateAPIToken(token string) error {
	if token == "" {
		return fmt.Errorf("%w: API token is required when enabled", ErrAPIUnavailable)
	}
	if len(token) > 4096 {
		return fmt.Errorf("%w: API token is invalid", ErrAPIUnavailable)
	}
	for _, character := range token {
		if character <= 0x20 || character == 0x7f {
			return fmt.Errorf("%w: API token is invalid", ErrAPIUnavailable)
		}
	}
	return nil
}

// APIClient makes narrowly-scoped reconciliation requests. Its public methods
// construct all API paths from validated identifiers, preventing caller input
// from escaping the Cloudflare v4 endpoint hierarchy.
type APIClient struct {
	config APIConfig
	base   *url.URL
	client *http.Client
}

// APIResult is a bounded response from a Cloudflare API operation. Body is
// returned only for a successful non-dry-run response and never contains the
// request token.
type APIResult struct {
	DryRun     bool
	Method     string
	Path       string
	StatusCode int
	Body       json.RawMessage
}

// NewAPIClient builds an opt-in client without making a request.
func NewAPIClient(config APIConfig) (*APIClient, error) {
	normalized, err := config.normalized()
	if err != nil {
		return nil, err
	}
	base, err := url.Parse(normalized.BaseURL)
	if err != nil {
		return nil, err
	}
	return &APIClient{
		config: normalized,
		base:   base,
		client: cloneNoRedirectClient(normalized.HTTPClient),
	}, nil
}

// ReconcileDNSRecord creates a record when recordID is empty, otherwise it
// updates the identified record. desired must marshal to a JSON object that
// Cloudflare's DNS endpoint accepts.
func (client *APIClient) ReconcileDNSRecord(ctx context.Context, zoneID, recordID string, desired any) (APIResult, error) {
	if !validCloudflareHexID(zoneID) {
		return APIResult{}, fmt.Errorf("%w: zone ID", ErrInvalidIdentifier)
	}
	segments := []string{"zones", zoneID, "dns_records"}
	method := http.MethodPost
	if recordID != "" {
		if !validCloudflareHexID(recordID) {
			return APIResult{}, fmt.Errorf("%w: DNS record ID", ErrInvalidIdentifier)
		}
		segments = append(segments, recordID)
		method = http.MethodPut
	}
	return client.perform(ctx, method, segments, desired)
}

// DeleteDNSRecord removes an explicitly identified record. Deletion is still
// blocked unless Enabled and APIToken were configured on the client.
func (client *APIClient) DeleteDNSRecord(ctx context.Context, zoneID, recordID string) (APIResult, error) {
	if !validCloudflareHexID(zoneID) {
		return APIResult{}, fmt.Errorf("%w: zone ID", ErrInvalidIdentifier)
	}
	if !validCloudflareHexID(recordID) {
		return APIResult{}, fmt.Errorf("%w: DNS record ID", ErrInvalidIdentifier)
	}
	return client.perform(ctx, http.MethodDelete, []string{"zones", zoneID, "dns_records", recordID}, nil)
}

// ReconcileTunnel updates an existing Cloudflare Tunnel. Tunnel operations
// need the account ID configured on APIConfig as well as a validated tunnel ID.
func (client *APIClient) ReconcileTunnel(ctx context.Context, tunnelID string, desired any) (APIResult, error) {
	if client == nil || !validCloudflareHexID(client.config.AccountID) {
		return APIResult{}, fmt.Errorf("%w: account ID", ErrInvalidIdentifier)
	}
	if !validTunnelID(tunnelID) {
		return APIResult{}, fmt.Errorf("%w: tunnel ID", ErrInvalidIdentifier)
	}
	return client.perform(ctx, http.MethodPut, []string{"accounts", client.config.AccountID, "cfd_tunnel", tunnelID}, desired)
}

// DeleteTunnel removes an explicitly identified Cloudflare Tunnel.
func (client *APIClient) DeleteTunnel(ctx context.Context, tunnelID string) (APIResult, error) {
	if client == nil || !validCloudflareHexID(client.config.AccountID) {
		return APIResult{}, fmt.Errorf("%w: account ID", ErrInvalidIdentifier)
	}
	if !validTunnelID(tunnelID) {
		return APIResult{}, fmt.Errorf("%w: tunnel ID", ErrInvalidIdentifier)
	}
	return client.perform(ctx, http.MethodDelete, []string{"accounts", client.config.AccountID, "cfd_tunnel", tunnelID}, nil)
}

func (client *APIClient) perform(ctx context.Context, method string, segments []string, desired any) (APIResult, error) {
	if client == nil || !client.config.Enabled || validateAPIToken(client.config.APIToken) != nil {
		return APIResult{}, ErrAPIUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var body []byte
	var err error
	if method != http.MethodDelete {
		if desired == nil {
			return APIResult{}, errors.New("Cloudflare API reconciliation payload is required")
		}
		body, err = json.Marshal(desired)
		if err != nil {
			return APIResult{}, errors.New("Cloudflare API reconciliation payload is invalid")
		}
		if len(body) == 0 || len(body) > maxAPIBodySize {
			return APIResult{}, errors.New("Cloudflare API reconciliation payload exceeds size limit")
		}
	}

	requestURL := client.endpoint(segments...)
	result := APIResult{DryRun: client.config.DryRun, Method: method, Path: requestURL.EscapedPath()}
	if client.config.DryRun {
		return result, nil
	}

	requestContext, cancel := context.WithTimeout(ctx, client.config.RequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, method, requestURL.String(), bytes.NewReader(body))
	if err != nil {
		return APIResult{}, errors.New("could not build Cloudflare API request")
	}
	request.Header.Set("Authorization", "Bearer "+client.config.APIToken)
	request.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.client.Do(request)
	if err != nil {
		return APIResult{}, errors.New("Cloudflare API request failed")
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxAPIBodySize+1))
	if err != nil || len(responseBody) > maxAPIBodySize {
		return APIResult{}, errors.New("Cloudflare API response is invalid")
	}
	result.StatusCode = response.StatusCode
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return APIResult{}, fmt.Errorf("Cloudflare API request returned status %d", response.StatusCode)
	}
	if !cloudflareEnvelopeSucceeded(responseBody) {
		return APIResult{}, errors.New("Cloudflare API request was not accepted")
	}
	result.Body = append(json.RawMessage(nil), responseBody...)
	return result, nil
}

func (client *APIClient) endpoint(segments ...string) *url.URL {
	endpoint := *client.base
	allSegments := append([]string{endpoint.Path}, segments...)
	endpoint.Path = path.Join(allSegments...)
	endpoint.RawPath = ""
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	return &endpoint
}

func cloneNoRedirectClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{Timeout: defaultRequestTimeout}
	}
	clone := *client
	clone.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &clone
}

func cloudflareEnvelopeSucceeded(body []byte) bool {
	if len(body) == 0 {
		return true
	}
	var envelope struct {
		Success *bool `json:"success"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return false
	}
	return envelope.Success != nil && *envelope.Success
}

func validCloudflareHexID(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, character := range value {
		if (character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F') {
			continue
		}
		return false
	}
	return true
}

func validTunnelID(value string) bool {
	if validCloudflareHexID(value) {
		return true
	}
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}
