package challenge

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strconv"
)

// RenderDynamicErrorPage generates HTML. Puzzle click/slider/word UIs are
// gone: browsers get a small worker that solves the same JSON agents POST.
func RenderDynamicErrorPage(ch *Challenge, status int, message string) string {
	if ch == nil || ch.Type == ChallengeNone || ch.Type != ChallengePoW {
		return renderSimpleError(status, message)
	}
	return renderPoWChallenge(ch, status, message)
}

func RenderChallengeJSON(ch *Challenge, status int, message string) []byte {
	if ch == nil || ch.Type != ChallengePoW {
		payload, _ := json.Marshal(map[string]any{
			"error":  message,
			"status": status,
		})
		return payload
	}
	payload, _ := json.Marshal(ch.Wire())
	return payload
}

func renderSimpleError(status int, message string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Request Blocked</title>
  <style>
    :root { color-scheme: light dark; }
    body { margin: 0; font: 16px/1.4 system-ui, sans-serif; display: grid; place-items: center; min-height: 100vh; background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); }
    .card { max-width: 500px; padding: 32px; background: white; border-radius: 16px; box-shadow: 0 20px 60px rgba(0,0,0,0.3); }
    h1 { margin: 0 0 12px; font-size: 24px; color: #333; }
    p { margin: 0 0 12px; color: #666; }
    code { background: #f5f5f5; padding: 2px 6px; border-radius: 4px; font-size: 14px; }
  </style>
</head>
<body>
  <div class="card">
    <h1>Request Blocked</h1>
    <p>%s</p>
    <p>Status: <code>%d</code></p>
  </div>
</body>
</html>`, template.HTMLEscapeString(message), status)
}

func renderPoWChallenge(ch *Challenge, status int, message string) string {
	wire, err := json.Marshal(ch.Wire())
	if err != nil {
		return renderSimpleError(status, message)
	}
	escaped := template.JSEscapeString(string(wire))
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Verification Required</title>
  <style>
    :root { color-scheme: light dark; }
    body { margin: 0; font: 16px/1.4 system-ui, sans-serif; display: grid; place-items: center; min-height: 100vh; background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); }
    .card { max-width: 520px; padding: 40px; background: white; border-radius: 16px; box-shadow: 0 20px 60px rgba(0,0,0,0.3); }
    h1 { margin: 0 0 8px; font-size: 24px; color: #333; }
    p { color: #666; }
    .status { font-size: 13px; color: #888; margin-top: 16px; }
    code { font-size: 12px; word-break: break-all; }
  </style>
</head>
<body>
  <div class="card">
    <h1>Verification Required</h1>
    <p>%s</p>
    <p id="status">Solving a short proof-of-work…</p>
    <div class="status">Status: %s · difficulty %s</div>
  </div>
  <script>
    const puzzle = JSON.parse("%s");
    const statusEl = document.getElementById("status");

    function hexToBytes(hex) {
      const out = new Uint8Array(hex.length / 2);
      for (let i = 0; i < out.length; i++) {
        out[i] = parseInt(hex.substr(i * 2, 2), 16);
      }
      return out;
    }

    function leadingZeroBits(bytes) {
      let bits = 0;
      for (let i = 0; i < bytes.length; i++) {
        const b = bytes[i];
        if (b === 0) { bits += 8; continue; }
        for (let bit = 7; bit >= 0; bit--) {
          if ((b & (1 << bit)) === 0) bits++;
          else return bits;
        }
      }
      return bits;
    }

    async function solve(p) {
      const mac = hexToBytes(p.mac);
      const buf = new Uint8Array(mac.length + 8);
      buf.set(mac, 0);
      const view = new DataView(buf.buffer);
      for (let counter = 0; ; counter++) {
        view.setUint32(mac.length, Math.floor(counter / 0x100000000), false);
        view.setUint32(mac.length + 4, counter >>> 0, false);
        const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", buf));
        if (leadingZeroBits(digest) >= p.difficulty) return counter;
      }
    }

    (async () => {
      try {
        const counter = await solve(puzzle);
        const body = JSON.stringify({ nonce: puzzle.nonce, counter: counter });
        const res = await fetch("/__netgoat/verify", {
          method: "POST",
          headers: { "Content-Type": "application/json", "Accept": "application/json" },
          body: body,
        });
        if (res.redirected) {
          window.location = res.url;
          return;
        }
        if (res.ok) {
          window.location.reload();
          return;
        }
        statusEl.textContent = "Verification failed. Reload to retry.";
      } catch (err) {
        statusEl.textContent = "Verification failed. Reload to retry.";
      }
    })();
  </script>
</body>
</html>`, template.HTMLEscapeString(message), strconv.Itoa(status), strconv.Itoa(ch.Difficulty), escaped)
}
