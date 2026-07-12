package debugoverlay

import (
	"strings"
	"testing"
)

func TestInjectOverlayHandlesUppercaseBodyTag(t *testing.T) {
	body := []byte("<html><BODY>hello</BODY></html>")
	out := string(InjectOverlay(body, &AnalysisInfo{RequestAllowed: true}))

	if !strings.Contains(out, "netgoat-debug-overlay") {
		t.Fatal("overlay was not injected")
	}
	if !strings.Contains(out, "</BODY>") {
		t.Fatal("original closing body tag case should be preserved")
	}
}
