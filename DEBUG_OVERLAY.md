# NetGoat Live AI Debug Overlay

## Overview

The NetGoat agent now includes a **live debug overlay** that displays real-time AI risk analysis and WAF decisions directly on web pages. This powerful feature helps you understand exactly what the AI is detecting and how requests are being processed.

## Features

### 🤖 AI Risk Analysis Display
- **Live Classification**: See what the AI model classified the request as
- **Risk Score Visualization**: Dynamic progress bar showing risk percentage
- **Processing Time**: Exact milliseconds the AI took to analyze
- **Threshold Comparison**: Visual indication of safety threshold
- **Verdict Display**: Clear indication if request was blocked or allowed

### 🛡️ WAF Analysis Display
- **Rule Matching**: Which WAF rules were triggered
- **Block Status**: Whether request was blocked by WAF
- **Rule Details**: Name and reason for any matches

### 📊 Request Details
- **Complete Request Info**: Method, host, path, target URL
- **Routing Information**: Where the request was proxied to
- **Cache Status**: Whether response was served from cache
- **Client IP**: Source IP address
- **Timestamp**: Exact time of request analysis

### 🎨 Visual Design
- **Floating Toggle Button**: Minimalist bottom-right corner button with goat emoji
- **Expandable Panel**: Slides up to show detailed analysis
- **Color-Coded Status**: Green for safe, orange for warning, red for blocked
- **Animated Indicators**: Pulsing status light and smooth transitions
- **Dark Theme**: Easy on the eyes with gradient backgrounds

## Configuration

### Enable in config.yml

```yaml
debug_overlay: true  # Show live AI analysis overlay on pages
```

### When to Use

**Enable for:**
- Development and testing
- Understanding AI model behavior
- Debugging WAF rules
- Training and demonstrations
- Security audits

**Disable for:**
- Production environments (unless debugging)
- Public-facing sites (exposes internal analysis)
- Performance-critical applications

## How It Works

### Request Flow

1. **Request Arrives** → Agent captures initial metadata
2. **AI Analysis** → If enabled, runs ML model prediction
3. **WAF Check** → Evaluates against Web Application Firewall rules
4. **Routing Decision** → Determines target backend
5. **Response Modification** → Injects debug overlay into HTML responses
6. **User Sees** → Live analysis displayed in corner of page

### Injection Process

The overlay is automatically injected into:
- ✅ HTML responses (`Content-Type: text/html`)
- ✅ Responses with `</body>` tag
- ❌ JSON, XML, images, or other non-HTML content

## Using the Overlay

### Opening the Debug Panel

1. Look for the **🐐 NetGoat AI** button in the bottom-right corner
2. Click the button to expand the analysis panel
3. Click again to collapse

### Understanding the Display

#### Status Indicator
- 🟢 **Green Pulse**: Request safe, low risk
- 🟠 **Orange Pulse**: Elevated risk but allowed
- 🔴 **Red Pulse**: High risk or blocked

#### AI Risk Score
```
┌─────────────────────────────────────┐
│  Risk Score: ████████░░░░░░  56.3%  │
│  Threshold:           70.0%         │
└─────────────────────────────────────┘
```
- **Blue Bar**: Very low risk (0-30%)
- **Green Bar**: Low risk (30-50%)
- **Yellow Bar**: Medium risk (50-70%)
- **Orange/Red Bar**: High risk (70-100%)

#### AI Classification Labels
Common labels you might see:
- ✅ `NORMAL` / `BENIGN` / `SAFE` - Normal traffic
- ⚠️ `SUSPICIOUS` / `WARNING` - Elevated but not blocked
- ⛔ `ANOMALY` / `MALICIOUS` / `ATTACK` - Blocked traffic

#### Final Decision Box
- **Green Box** with "✅ REQUEST ALLOWED" - Request proceeded normally
- **Red Box** with "⛔ REQUEST BLOCKED" - Request was denied

## Testing the Feature

### 1. Enable AI Anomaly Detection

```yaml
# config.yml
debug_overlay: true

anomaly:
  enabled: true
  threshold: 0.7
  model_path: "ai/goatai.keras"
  scaler_path: "ai/scaler.pkl"
  python_script: "ai/model_server.py"
  feature_header: "X-GoatAI-Features"
```

### 2. Send Test Requests

```bash
# Normal request (should show low risk)
curl -H "X-GoatAI-Features: 0.1,0.2,0.05,0.3,10,200,5,0" \
     http://localhost:8080/

# Suspicious request (elevated risk)
curl -H "X-GoatAI-Features: 0.6,0.7,0.65,0.8,50,500,25,1" \
     http://localhost:8080/

# Malicious request (should block)
curl -H "X-GoatAI-Features: 0.9,0.95,0.88,0.92,100,999,50,1" \
     http://localhost:8080/
```

### 3. View in Browser

Visit any page served through the proxy:
1. Open `http://localhost:8080` in browser
2. Look for the goat button in bottom-right
3. Click to see the analysis
4. Try different URLs to see how analysis changes

### 4. Test WAF Rules

```bash
# Trigger WAF rule
curl http://localhost:8080/admin
curl "http://localhost:8080/api?search=UNION+SELECT"
```

The overlay will show which WAF rule triggered.

## Example Analysis Output

```
┌────────────────────────────────────┐
│ 🛡️ Request Analysis          ALLOWED │
│ ⏱️ 10:30:45   📍 192.168.1.100     │
├────────────────────────────────────┤
│ 📡 Request Details                 │
│   Method:   GET                    │
│   Host:     example.com            │
│   Path:     /api/users             │
│   Target:   http://backend:3000    │
├────────────────────────────────────┤
│ 🔥 WAF Analysis                    │
│   ✅ No threats detected           │
├────────────────────────────────────┤
│ 🤖 AI Risk Analysis                │
│   Classification: NORMAL           │
│   Risk Score: ████░░░░░░ 23.4%     │
│   Threshold:           70.0%       │
│   Processing: 12ms                 │
│                                    │
│   ✅ VERIFIED SAFE                 │
│   Request within normal parameters │
├────────────────────────────────────┤
│ 📊 Final Decision                  │
│   ✅ REQUEST ALLOWED               │
└────────────────────────────────────┘
```

## Performance Impact

### Resource Usage
- **CPU**: <1ms for overlay injection
- **Memory**: ~50KB per injected page
- **Network**: ~15KB additional HTML/JS/CSS per page

### Optimization Tips
1. **Disable in Production**: Only enable when needed
2. **Cache Aware**: Overlay respects cache settings
3. **HTML Only**: Only injects into HTML responses
4. **Minimal JavaScript**: Uses vanilla JS, no frameworks

## Troubleshooting

### Overlay Not Appearing

**Check:**
1. ✅ `debug_overlay: true` in config.yml
2. ✅ Response is HTML (not JSON/API)
3. ✅ Response has `</body>` tag
4. ✅ Agent is running latest code

**Debug:**
```bash
# Check agent logs
grep "debug_overlay" agent.log

# Verify config
cat config.yml | grep debug_overlay
```

### AI Analysis Shows Error

**Common Issues:**
1. AI model not loaded (check model paths)
2. Python server not running
3. No X-GoatAI-Features header provided
4. Invalid CSV format in header

**Solution:**
Check agent logs for AI initialization:
```
INFO Anomaly detection configured model=ai/goatai.keras
```

### Overlay Appears on Wrong Pages

The overlay injects into ALL HTML responses. To exclude:
- Return JSON responses (`Content-Type: application/json`)
- Use API endpoints without HTML
- Disable overlay for specific routes (not yet implemented)

## Advanced Usage

### Custom Feature Vectors

Send custom AI features for testing:

```javascript
fetch('/api/endpoint', {
  headers: {
    'X-GoatAI-Features': '0.5,0.6,0.4,0.7,25,300,10,0'
  }
})
```

Feature vector format:
```
[feature1, feature2, feature3, feature4, count1, count2, size, flag]
```

### Monitoring AI Decisions

Watch real-time AI decisions:
```bash
# Follow agent logs
tail -f agent.log | grep "anomaly prediction"

# Count blocks
tail -f agent.log | grep "Blocked by local anomaly" | wc -l
```

### Integration with Frontend

The overlay can be accessed programmatically:

```javascript
// Check if overlay exists
if (document.getElementById('netgoat-debug-overlay')) {
  console.log('NetGoat AI monitoring active');
  
  // Auto-open panel
  document.getElementById('netgoat-debug-toggle').click();
}
```

## Security Considerations

### ⚠️ Important Warnings

1. **Do Not Use in Production** (unless specifically debugging)
   - Exposes internal security analysis
   - Reveals AI model behavior
   - Shows backend routing information

2. **Sensitive Information**
   - Client IPs are displayed
   - Backend URLs are visible
   - AI model thresholds exposed

3. **Recommended Usage**
   - Development environments only
   - Internal security testing
   - Controlled demonstration environments
   - Behind additional authentication

### Best Practices

```yaml
# Development
debug_overlay: true

# Staging
debug_overlay: false  # Only enable for specific tests

# Production  
debug_overlay: false  # Always disabled
```

## Future Enhancements

Planned features:
- [ ] Toggle overlay via query parameter (`?debug=1`)
- [ ] Export analysis as JSON
- [ ] Historical request timeline
- [ ] Real-time statistics dashboard
- [ ] Custom theme colors
- [ ] Draggable/resizable panel
- [ ] WebSocket live updates
- [ ] Rule explanation tooltips

## Screenshots

### Allowed Request
![Allowed Request](docs/overlay-allowed.png)

### Blocked Request
![Blocked Request](docs/overlay-blocked.png)

## Support

For issues or questions:
1. Check agent logs: `tail -f agent.log`
2. Verify configuration: `cat config.yml`
3. Test with simple HTML page first
4. Review this documentation

## Summary

The live AI debug overlay provides unprecedented visibility into NetGoat's security analysis. Use it to understand, debug, and demonstrate the AI-powered threat detection system in action! 🐐✨
