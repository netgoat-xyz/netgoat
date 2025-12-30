package challenge

import (
	"fmt"
	"html/template"
)

// RenderDynamicErrorPage generates HTML with embedded challenge
func RenderDynamicErrorPage(ch *Challenge, status int, message string) string {
	if ch == nil || ch.Type == ChallengeNone {
		return renderSimpleError(status, message)
	}

	switch ch.Type {
	case ChallengeText:
		return renderTextChallenge(ch, status, message)
	case ChallengeClick:
		return renderClickChallenge(ch, status, message)
	case ChallengeSlider:
		return renderSliderChallenge(ch, status, message)
	default:
		return renderSimpleError(status, message)
	}
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
    <h1>🚫 Request Blocked</h1>
    <p>%s</p>
    <p>Status: <code>%d</code></p>
  </div>
</body>
</html>`, template.HTMLEscapeString(message), status)
}

func renderTextChallenge(ch *Challenge, status int, message string) string {
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
    .bot-id { font-size: 11px; color: #999; font-family: monospace; margin-bottom: 16px; }
    .challenge { margin: 24px 0; padding: 20px; background: #f8f9fa; border-radius: 8px; border-left: 4px solid #667eea; }
    .word { display: inline-block; padding: 8px 16px; margin: 4px; background: white; border: 2px solid #667eea; border-radius: 8px; font-size: 20px; font-weight: bold; letter-spacing: 3px; color: #667eea; }
    input { width: 100%%; padding: 12px; font-size: 16px; border: 2px solid #ddd; border-radius: 8px; box-sizing: border-box; margin-top: 12px; }
    button { width: 100%%; padding: 12px; background: #667eea; color: white; border: none; border-radius: 8px; font-size: 16px; font-weight: 600; cursor: pointer; margin-top: 12px; }
    button:hover { background: #5568d3; }
    .suspicion { font-size: 12px; color: #999; margin-top: 12px; }
  </style>
</head>
<body>
  <div class="card">
    <h1>🤖 Verification Required</h1>
    <div class="bot-id">Bot ID: %s</div>
    <p style="color: #666; margin-bottom: 8px;">Your request was flagged by our security system.</p>
    <div class="challenge">
      <p style="margin: 0 0 12px; font-weight: 600; color: #333;">Type the word shown below:</p>
      <div class="word">%s</div>
    </div>
    <form method="POST" action="/__netgoat/verify">
      <input type="hidden" name="challenge_id" value="%s"/>
      <input type="text" name="answer" placeholder="Enter the word" autocomplete="off" autofocus required/>
      <button type="submit">Verify</button>
    </form>
    <div class="suspicion">Suspicion Score: %d/100 | Status: %d</div>
  </div>
</body>
</html>`, ch.ID, ch.Answer, ch.ID, ch.Suspicion, status)
}

func renderClickChallenge(ch *Challenge, status int, message string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Verification Required</title>
  <style>
    :root { color-scheme: light dark; }
    body { margin: 0; font: 16px/1.4 system-ui, sans-serif; display: grid; place-items: center; min-height: 100vh; background: linear-gradient(135deg, #f093fb 0%%, #f5576c 100%%); }
    .card { max-width: 520px; padding: 40px; background: white; border-radius: 16px; box-shadow: 0 20px 60px rgba(0,0,0,0.3); }
    h1 { margin: 0 0 8px; font-size: 24px; color: #333; }
    .bot-id { font-size: 11px; color: #999; font-family: monospace; margin-bottom: 16px; }
    .challenge { margin: 24px 0; }
    .grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
    .box { aspect-ratio: 1; background: #f8f9fa; border: 3px solid #ddd; border-radius: 8px; cursor: pointer; display: flex; align-items: center; justify-content: center; font-size: 32px; transition: all 0.2s; }
    .box:hover { border-color: #f5576c; transform: scale(1.05); }
    .box.selected { background: #f5576c; border-color: #f5576c; color: white; }
    button { width: 100%%; padding: 12px; background: #f5576c; color: white; border: none; border-radius: 8px; font-size: 16px; font-weight: 600; cursor: pointer; margin-top: 16px; }
    button:hover { background: #e04858; }
    .suspicion { font-size: 12px; color: #999; margin-top: 12px; }
  </style>
</head>
<body>
  <div class="card">
    <h1>🎯 Click Verification</h1>
    <div class="bot-id">Bot ID: %s</div>
    <p style="color: #666; margin-bottom: 8px;">Select all boxes containing <strong>🚀</strong></p>
    <div class="challenge">
      <div class="grid" id="grid"></div>
    </div>
    <form method="POST" action="/__netgoat/verify" id="verifyForm">
      <input type="hidden" name="challenge_id" value="%s"/>
      <input type="hidden" name="answer" id="answer" value=""/>
      <button type="submit">Verify Selection</button>
    </form>
    <div class="suspicion">Suspicion Score: %d/100 | Status: %d</div>
  </div>
  <script>
    const correct = "%s".split(",").map(x => parseInt(x));
    const selected = new Set();
    const grid = document.getElementById("grid");
    const emojis = ["🌟", "🎈", "🎨", "🎭", "🎪", "🎬", "🎮", "🎯", "🎲"];
    
    for (let i = 0; i < 9; i++) {
      const box = document.createElement("div");
      box.className = "box";
      box.textContent = correct.includes(i) ? "🚀" : emojis[i];
      box.onclick = () => {
        if (selected.has(i)) {
          selected.delete(i);
          box.classList.remove("selected");
        } else {
          selected.add(i);
          box.classList.add("selected");
        }
        document.getElementById("answer").value = Array.from(selected).sort().join(",");
      };
      grid.appendChild(box);
    }
  </script>
</body>
</html>`, ch.ID, ch.ID, ch.Suspicion, status, ch.Answer)
}

func renderSliderChallenge(ch *Challenge, status int, message string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Verification Required</title>
  <style>
    :root { color-scheme: light dark; }
    body { margin: 0; font: 16px/1.4 system-ui, sans-serif; display: grid; place-items: center; min-height: 100vh; background: linear-gradient(135deg, #fa709a 0%%, #fee140 100%%); }
    .card { max-width: 520px; padding: 40px; background: white; border-radius: 16px; box-shadow: 0 20px 60px rgba(0,0,0,0.3); }
    h1 { margin: 0 0 8px; font-size: 24px; color: #333; }
    .bot-id { font-size: 11px; color: #999; font-family: monospace; margin-bottom: 16px; }
    .challenge { margin: 24px 0; }
    .puzzle-container { position: relative; width: 100%%; height: 200px; background: linear-gradient(90deg, #fa709a 0%%, #fee140 100%%); border-radius: 12px; overflow: hidden; }
    .puzzle-piece { position: absolute; width: 60px; height: 60px; background: white; border: 3px solid #333; border-radius: 8px; cursor: grab; box-shadow: 0 4px 12px rgba(0,0,0,0.2); display: flex; align-items: center; justify-content: center; font-size: 24px; }
    .puzzle-piece:active { cursor: grabbing; }
    .target-zone { position: absolute; right: 20px; top: 70px; width: 70px; height: 70px; border: 3px dashed #333; border-radius: 8px; background: rgba(255,255,255,0.3); }
    button { width: 100%%; padding: 12px; background: #fa709a; color: white; border: none; border-radius: 8px; font-size: 16px; font-weight: 600; cursor: pointer; margin-top: 16px; }
    button:hover { background: #e8638a; }
    .suspicion { font-size: 12px; color: #999; margin-top: 12px; }
  </style>
</head>
<body>
  <div class="card">
    <h1>🧩 Puzzle Verification</h1>
    <div class="bot-id">Bot ID: %s</div>
    <p style="color: #666; margin-bottom: 8px;">Drag the puzzle piece to the target zone</p>
    <div class="challenge">
      <div class="puzzle-container">
        <div class="target-zone"></div>
        <div class="puzzle-piece" id="piece" style="left: 20px; top: 70px;">🔒</div>
      </div>
    </div>
    <form method="POST" action="/__netgoat/verify" id="verifyForm">
      <input type="hidden" name="challenge_id" value="%s"/>
      <input type="hidden" name="answer" id="answer" value=""/>
      <button type="submit">Verify</button>
    </form>
    <div class="suspicion">Suspicion Score: %d/100 | Status: %d</div>
  </div>
  <script>
    const piece = document.getElementById("piece");
    const target = "%s";
    let solved = false;
    
    piece.onmousedown = (e) => {
      e.preventDefault();
      const shiftX = e.clientX - piece.getBoundingClientRect().left;
      const shiftY = e.clientY - piece.getBoundingClientRect().top;
      
      const move = (e) => {
        const container = piece.parentElement.getBoundingClientRect();
        let x = e.clientX - container.left - shiftX;
        let y = e.clientY - container.top - shiftY;
        x = Math.max(0, Math.min(x, container.width - 60));
        y = Math.max(0, Math.min(y, container.height - 60));
        piece.style.left = x + "px";
        piece.style.top = y + "px";
        
        // Check if close to target (right side)
        if (x > container.width - 100 && y > 50 && y < 110) {
          piece.style.background = "#4ade80";
          piece.textContent = "✓";
          solved = true;
          document.getElementById("answer").value = target;
        } else {
          piece.style.background = "white";
          piece.textContent = "🔒";
          solved = false;
          document.getElementById("answer").value = "";
        }
      };
      
      const up = () => {
        document.removeEventListener("mousemove", move);
        document.removeEventListener("mouseup", up);
      };
      
      document.addEventListener("mousemove", move);
      document.addEventListener("mouseup", up);
    };
  </script>
</body>
</html>`, ch.ID, ch.ID, ch.Suspicion, status, ch.Answer)
}
