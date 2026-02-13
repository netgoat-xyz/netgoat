package debugoverlay

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"
)

// AnalysisInfo contains all debug information for a request
type AnalysisInfo struct {
	RequestID string
	Timestamp time.Time
	ClientIP  string
	Host      string
	Path      string
	Method    string

	// WAF Analysis
	WAFChecked     bool
	WAFBlocked     bool
	WAFRuleName    string
	WAFRuleMatched string

	// AI Analysis
	AIEnabled      bool
	AIChecked      bool
	AILabel        string
	AIScore        float64
	AIThreshold    float64
	AIBlocked      bool
	AIError        string
	AIProcessingMs int64

	// Routing
	TargetURL string
	CacheHit  bool

	// Overall Decision
	RequestAllowed bool
	BlockReason    string
}

// InjectOverlay injects a debug overlay into HTML responses
func InjectOverlay(htmlBody []byte, info *AnalysisInfo) []byte {
	if info == nil {
		return htmlBody
	}

	// Only inject into HTML responses
	bodyStr := string(htmlBody)
	if !strings.Contains(strings.ToLower(bodyStr), "</body>") {
		return htmlBody
	}

	overlay := generateOverlayHTML(info)

	// Inject before closing body tag
	modifiedBody := strings.Replace(bodyStr, "</body>", overlay+"</body>", 1)
	return []byte(modifiedBody)
}

func generateOverlayHTML(info *AnalysisInfo) string {
	tmpl := `
<!-- NetGoat Debug Overlay -->
<div id="netgoat-debug-overlay" style="position: fixed; bottom: 20px; right: 20px; z-index: 999999; font-family: 'Monaco', 'Menlo', monospace; font-size: 12px;">
	<div id="netgoat-debug-toggle" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 10px 15px; border-radius: 8px; cursor: pointer; box-shadow: 0 4px 15px rgba(0,0,0,0.3); display: flex; align-items: center; gap: 8px;">
		<span style="font-size: 16px;">🐐</span>
		<span style="font-weight: bold;">NetGoat AI</span>
		<span id="netgoat-status-indicator" style="width: 8px; height: 8px; border-radius: 50%; background: {{.StatusColor}}; animation: pulse 2s infinite;"></span>
	</div>
	
	<div id="netgoat-debug-panel" style="display: none; position: absolute; bottom: 60px; right: 0; background: #1a1a2e; color: #eee; border-radius: 12px; box-shadow: 0 8px 30px rgba(0,0,0,0.5); width: 420px; max-height: 600px; overflow-y: auto; backdrop-filter: blur(10px);">
		<div style="padding: 20px; border-bottom: 2px solid #16213e;">
			<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
				<h3 style="margin: 0; font-size: 16px; color: #fff; display: flex; align-items: center; gap: 8px;">
					<span>🛡️</span> Request Analysis
				</h3>
				<span style="background: {{.StatusBadgeColor}}; padding: 4px 10px; border-radius: 12px; font-size: 10px; font-weight: bold; color: white;">{{.StatusText}}</span>
			</div>
			<div style="font-size: 10px; color: #888; display: flex; gap: 12px;">
				<span>⏱️ {{.Timestamp}}</span>
				<span>📍 {{.ClientIP}}</span>
			</div>
		</div>
		
		<!-- Request Info -->
		<div style="padding: 16px; border-bottom: 1px solid #2a2a3e;">
			<div style="font-weight: bold; color: #ffa500; margin-bottom: 8px; display: flex; align-items: center; gap: 6px;">
				<span>📡</span> Request Details
			</div>
			<div style="font-size: 11px; line-height: 1.6;">
				<div style="display: grid; grid-template-columns: 80px 1fr; gap: 6px;">
					<span style="color: #888;">Method:</span>
					<span style="color: #00ff88; font-weight: bold;">{{.Method}}</span>
					<span style="color: #888;">Host:</span>
					<span style="color: #fff;">{{.Host}}</span>
					<span style="color: #888;">Path:</span>
					<span style="color: #fff; word-break: break-all;">{{.Path}}</span>
					<span style="color: #888;">Target:</span>
					<span style="color: #66d9ef; word-break: break-all;">{{.TargetURL}}</span>
				</div>
			</div>
		</div>
		
		{{if .WAFChecked}}
		<!-- WAF Analysis -->
		<div style="padding: 16px; border-bottom: 1px solid #2a2a3e; background: {{.WAFBackground}};">
			<div style="font-weight: bold; color: #ff6b6b; margin-bottom: 8px; display: flex; align-items: center; gap: 6px;">
				<span>🔥</span> WAF Analysis
			</div>
			<div style="font-size: 11px; line-height: 1.6;">
				{{if .WAFBlocked}}
				<div style="background: #ff4757; padding: 8px; border-radius: 6px; margin-bottom: 8px; border-left: 4px solid #ff0000;">
					<div style="font-weight: bold; color: white;">⛔ REQUEST BLOCKED</div>
					<div style="color: #ffe0e0; margin-top: 4px;">Rule: {{.WAFRuleName}}</div>
				</div>
				{{else}}
				<div style="color: #00ff88; display: flex; align-items: center; gap: 6px;">
					<span>✅</span> No threats detected
				</div>
				{{end}}
			</div>
		</div>
		{{end}}
		
		{{if .AIEnabled}}
		<!-- AI Analysis -->
		<div style="padding: 16px; border-bottom: 1px solid #2a2a3e; background: {{.AIBackground}};">
			<div style="font-weight: bold; color: #a29bfe; margin-bottom: 8px; display: flex; align-items: center; gap: 6px;">
				<span>🤖</span> AI Risk Analysis
			</div>
			{{if .AIChecked}}
			<div style="font-size: 11px; line-height: 1.6;">
				<div style="display: grid; grid-template-columns: 100px 1fr; gap: 8px; margin-bottom: 12px;">
					<span style="color: #888;">Classification:</span>
					<span style="color: {{.AILabelColor}}; font-weight: bold;">{{.AILabel}}</span>
					
					<span style="color: #888;">Risk Score:</span>
					<div style="display: flex; align-items: center; gap: 8px;">
						<div style="flex: 1; background: #2a2a3e; border-radius: 10px; height: 16px; overflow: hidden; position: relative;">
							<div style="background: {{.AIScoreColor}}; width: {{.AIScorePercent}}%; height: 100%; transition: width 0.3s ease;"></div>
							<div style="position: absolute; top: 0; left: 0; right: 0; bottom: 0; display: flex; align-items: center; justify-content: center; font-size: 10px; font-weight: bold; color: white; text-shadow: 0 1px 2px rgba(0,0,0,0.5);">
								{{.AIScoreDisplay}}
							</div>
						</div>
					</div>
					
					<span style="color: #888;">Threshold:</span>
					<span style="color: #ffa500;">{{.AIThresholdDisplay}}</span>
					
					<span style="color: #888;">Processing:</span>
					<span style="color: #888;">{{.AIProcessingMs}}ms</span>
				</div>
				
				{{if .AIBlocked}}
				<div style="background: #ff4757; padding: 8px; border-radius: 6px; border-left: 4px solid #ff0000;">
					<div style="font-weight: bold; color: white;">⛔ BLOCKED BY AI</div>
					<div style="color: #ffe0e0; margin-top: 4px; font-size: 10px;">High-risk behavior detected</div>
				</div>
				{{else}}
				<div style="background: #2ecc71; padding: 8px; border-radius: 6px; border-left: 4px solid #27ae60;">
					<div style="font-weight: bold; color: white;">✅ VERIFIED SAFE</div>
					<div style="color: #e8f8f0; margin-top: 4px; font-size: 10px;">Request within normal parameters</div>
				</div>
				{{end}}
			</div>
			{{else}}
			<div style="color: #888; font-size: 11px;">
				{{if ne .AIError ""}}
				<div style="background: #f39c12; padding: 8px; border-radius: 6px; color: white;">
					⚠️ Error: {{.AIError}}
				</div>
				{{else}}
				<div>ℹ️ No AI features provided (use X-GoatAI-Features header)</div>
				{{end}}
			</div>
			{{end}}
		</div>
		{{end}}
		
		<!-- Overall Status -->
		<div style="padding: 16px;">
			<div style="font-weight: bold; color: #74b9ff; margin-bottom: 8px; display: flex; align-items: center; gap: 6px;">
				<span>📊</span> Final Decision
			</div>
			<div style="font-size: 11px; line-height: 1.6;">
				{{if .RequestAllowed}}
				<div style="background: linear-gradient(135deg, #00b894, #00cec9); padding: 12px; border-radius: 8px; color: white; text-align: center; font-weight: bold;">
					✅ REQUEST ALLOWED
				</div>
				{{else}}
				<div style="background: linear-gradient(135deg, #d63031, #ff7675); padding: 12px; border-radius: 8px; color: white; text-align: center; font-weight: bold;">
					⛔ REQUEST BLOCKED
					<div style="font-size: 10px; margin-top: 4px; opacity: 0.9;">Reason: {{.BlockReason}}</div>
				</div>
				{{end}}
			</div>
		</div>
	</div>
</div>

<style>
@keyframes pulse {
	0%, 100% { opacity: 1; }
	50% { opacity: 0.5; }
}
#netgoat-debug-panel::-webkit-scrollbar {
	width: 8px;
}
#netgoat-debug-panel::-webkit-scrollbar-track {
	background: #16213e;
}
#netgoat-debug-panel::-webkit-scrollbar-thumb {
	background: #667eea;
	border-radius: 4px;
}
#netgoat-debug-panel::-webkit-scrollbar-thumb:hover {
	background: #764ba2;
}
</style>

<script>
(function() {
	const toggle = document.getElementById('netgoat-debug-toggle');
	const panel = document.getElementById('netgoat-debug-panel');
	
	toggle.addEventListener('click', function() {
		if (panel.style.display === 'none') {
			panel.style.display = 'block';
			panel.style.animation = 'slideIn 0.3s ease-out';
		} else {
			panel.style.display = 'none';
		}
	});
	
	// Add animation
	const style = document.createElement('style');
	style.textContent = '@keyframes slideIn { from { opacity: 0; transform: translateY(20px); } to { opacity: 1; transform: translateY(0); } }';
	document.head.appendChild(style);
})();
</script>
`

	t := template.Must(template.New("overlay").Parse(tmpl))

	data := map[string]interface{}{
		"Timestamp":      info.Timestamp.Format("15:04:05"),
		"ClientIP":       info.ClientIP,
		"Host":           info.Host,
		"Path":           info.Path,
		"Method":         info.Method,
		"TargetURL":      info.TargetURL,
		"RequestAllowed": info.RequestAllowed,
		"BlockReason":    info.BlockReason,

		// Status
		"StatusColor":      getStatusColor(info),
		"StatusBadgeColor": getStatusBadgeColor(info),
		"StatusText":       getStatusText(info),

		// WAF
		"WAFChecked":    info.WAFChecked,
		"WAFBlocked":    info.WAFBlocked,
		"WAFRuleName":   info.WAFRuleName,
		"WAFBackground": getWAFBackground(info),

		// AI
		"AIEnabled":          info.AIEnabled,
		"AIChecked":          info.AIChecked,
		"AILabel":            info.AILabel,
		"AIScore":            info.AIScore,
		"AIScoreDisplay":     fmt.Sprintf("%.1f%%", info.AIScore*100),
		"AIScorePercent":     fmt.Sprintf("%.0f", info.AIScore*100),
		"AIScoreColor":       getAIScoreColor(info.AIScore),
		"AILabelColor":       getAILabelColor(info),
		"AIThreshold":        info.AIThreshold,
		"AIThresholdDisplay": fmt.Sprintf("%.1f%%", info.AIThreshold*100),
		"AIBlocked":          info.AIBlocked,
		"AIError":            info.AIError,
		"AIProcessingMs":     info.AIProcessingMs,
		"AIBackground":       getAIBackground(info),
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return ""
	}

	return buf.String()
}

func getStatusColor(info *AnalysisInfo) string {
	if !info.RequestAllowed {
		return "#ff4757"
	}
	if info.AIBlocked || info.WAFBlocked {
		return "#ff4757"
	}
	if info.AIScore > info.AIThreshold*0.7 {
		return "#ffa500"
	}
	return "#00ff88"
}

func getStatusBadgeColor(info *AnalysisInfo) string {
	if !info.RequestAllowed {
		return "#d63031"
	}
	return "#00b894"
}

func getStatusText(info *AnalysisInfo) string {
	if !info.RequestAllowed {
		return "BLOCKED"
	}
	return "ALLOWED"
}

func getWAFBackground(info *AnalysisInfo) string {
	if info.WAFBlocked {
		return "rgba(255, 71, 87, 0.1)"
	}
	return "transparent"
}

func getAIBackground(info *AnalysisInfo) string {
	if info.AIBlocked {
		return "rgba(255, 71, 87, 0.1)"
	}
	if info.AIScore > info.AIThreshold*0.7 {
		return "rgba(255, 165, 0, 0.1)"
	}
	return "rgba(46, 204, 113, 0.05)"
}

func getAIScoreColor(score float64) string {
	if score >= 0.7 {
		return "linear-gradient(90deg, #ff4757, #ff6348)"
	}
	if score >= 0.5 {
		return "linear-gradient(90deg, #ffa502, #ff6348)"
	}
	if score >= 0.3 {
		return "linear-gradient(90deg, #feca57, #ffa502)"
	}
	return "linear-gradient(90deg, #00d2d3, #1dd1a1)"
}

func getAILabelColor(info *AnalysisInfo) string {
	label := strings.ToLower(info.AILabel)
	if strings.Contains(label, "attack") || strings.Contains(label, "malicious") || strings.Contains(label, "anom") {
		return "#ff4757"
	}
	if strings.Contains(label, "suspi") || strings.Contains(label, "warning") {
		return "#ffa502"
	}
	return "#00ff88"
}
