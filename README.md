# netgoat

Reverse proxy with WAF, honeypot, ZeroTrust login, and AI-based anomaly detection.

## New Features

- Custom error pages:
  - Default error page via `custom_error_page`.
  - Per-domain pages via `error_pages.domain`.
  - Per-path pages (longest prefix) via `error_pages.path`.
- AI anomaly detection (Hugging Face): block requests when the model flags anomalous CSV features.

## Configure

Edit `config.yml`:

```yaml
# Serve this HTML for error responses (optional default)
custom_error_page: "public/error.html"

# Optional: fine-grained error pages per domain or path
# Path rules use longest-prefix match and override domain and default.
error_pages:
  domain:
    # "app.example.com": "public/app-error.html"
    # "admin.example.com": "public/admin-error.html"
  path:
    # "/admin": "public/admin-error.html"
    # "/shop": "public/shop-error.html"

# Anomaly detection via Hugging Face (optional)
anomaly:
  enabled: true
  threshold: 0.7
  model: "netgoat-ai/GoatAI"
  # token may also be provided via env HUGGINGFACE_TOKEN or HUGGINGFACEHUB_API_TOKEN
  # huggingface_token: "${HUGGINGFACE_TOKEN}"
  feature_header: "X-GoatAI-Features"
```

Provide a Hugging Face API token either in `config.yml` as `anomaly.huggingface_token` or via environment variables `HUGGINGFACE_TOKEN` or `HUGGINGFACEHUB_API_TOKEN`.

## Run

```bash
# Build
go build ./...

# Run (HTTP on :8080 by default)
./netgoat
```

## How anomaly detection works

- If `anomaly.enabled` is true, the proxy looks for a CSV of features in the request:
  - Header `X-GoatAI-Features` (override with `feature_header`), or
  - Query param `goatai`.
- The CSV must contain, in order:
  1. Flow Duration
  2. Total Fwd Packets
  3. Total Backward Packets
  4. Packet Length Mean
  5. Flow IAT Mean
  6. Fwd Flag Count
- The string is sent to Hugging Face Inference API for model `netgoat-ai/GoatAI`.
- If the model returns a score >= `anomaly.threshold` for an anomalous label (or label explicitly includes "anom"/"malicious"/"attack"), the request is blocked with HTTP 403. If `custom_error_page` is set, that HTML is served instead of a plain error.

## Quick test

With the server running and anomaly enabled, send a request with features:

```bash
curl -H "X-GoatAI-Features: 10,5,2,123.4,56.7,1" http://localhost:8080/some/path
```

If the model classifies the vector as anomalous over the threshold, you will get a 403 with the custom error page (if configured).

## Notes

- If no features header/param is present, the AI step is skipped and normal WAF rules apply.
- The default custom error page lives at `public/error.html`; you can override per domain/path.
