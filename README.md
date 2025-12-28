# netgoat

Reverse proxy with WAF, honeypot, ZeroTrust login, and local AI-based anomaly detection using Keras + sklearn.

## New Features

- Custom error pages:
  - Default error page via `custom_error_page`.
  - Per-domain pages via `error_pages.domain`.
  - Per-path pages (longest prefix) via `error_pages.path`.
- Dynamic error pages with:
  - Bot detection (suspicious user-agents flagged for challenges).
  - Challenge system: text/click/puzzle CAPTCHAs based on suspicion level.
- Local AI anomaly detection: Keras model + sklearn scaler (no remote API calls).

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

# Local anomaly detection with Keras + sklearn scaler
anomaly:
  enabled: false
  threshold: 0.7
  model_path: "ai/goatai.keras"
  scaler_path: "ai/scaler.pkl"
  python_script: "ai/model_server.py"
  feature_header: "X-GoatAI-Features"
```

## Requirements

- Python 3 with TensorFlow and scikit-learn:
  ```bash
  pip install tensorflow scikit-learn
  ```

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
- The string is passed to a local Python subprocess (`ai/model_server.py`) which uses the Keras model and sklearn scaler.
- If the model returns a score >= `anomaly.threshold` for an anomalous label, the request is blocked with HTTP 403. If `custom_error_page` is set, that HTML is served instead of a plain error.

## Challenge System

Dynamic error pages detect suspicious user-agents and issue challenges:
- **Text CAPTCHA**: For slightly suspicious requests.
- **Click CAPTCHA**: For more suspicious requests.
- **Puzzle CAPTCHA**: For highly suspicious requests.

Challenges are issued at `/__netgoat/verify` and verified server-side.

## Quick test

With the server running and anomaly enabled, send a request with features:

```bash
curl -H "X-GoatAI-Features: 10,5,2,123.4,56.7,1" http://localhost:8080/some/path
```

If the model classifies the vector as anomalous over the threshold, you will get a 403 with the custom error page (if configured) and possibly a challenge.

## Notes

- If no features header/param is present, the AI step is skipped and normal WAF rules apply.
- The default custom error page lives at `public/error.html`; you can override per domain/path.
- The Python model server runs as a subprocess and is automatically cleaned up on shutdown.

