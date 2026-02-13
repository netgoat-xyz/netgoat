#!/bin/bash

# NetGoat AI Debug Overlay Quick Start Script

set -e

echo "🐐 NetGoat AI Debug Overlay Quick Start"
echo "========================================"
echo ""

# Check if we're in the agent directory
if [ ! -f "main.go" ]; then
    echo "❌ Error: Please run this script from the agent directory"
    exit 1
fi

# Step 1: Build the agent
echo "📦 Step 1: Building agent..."
go build -o agent .
if [ $? -eq 0 ]; then
    echo "✅ Agent built successfully"
else
    echo "❌ Build failed"
    exit 1
fi

# Step 2: Check config
echo ""
echo "⚙️  Step 2: Checking configuration..."
if grep -q "debug_overlay: true" config.yml; then
    echo "✅ Debug overlay is enabled"
else
    echo "⚠️  Debug overlay is not enabled in config.yml"
    echo "   Add 'debug_overlay: true' to enable it"
fi

# Step 3: Add test route to database
echo ""
echo "🗄️  Step 3: Setting up test route..."
if [ ! -d "database" ]; then
    mkdir -p database
fi

# Create a simple test backend using Python
echo "🌐 Step 4: Setting up test backend..."
cat > /tmp/netgoat_test_backend.py << 'EOF'
#!/usr/bin/env python3
from http.server import HTTPServer, SimpleHTTPRequestHandler
import os

class CORSRequestHandler(SimpleHTTPRequestHandler):
    def end_headers(self):
        self.send_header('Access-Control-Allow-Origin', '*')
        super().end_headers()
    
    def do_GET(self):
        # Serve the test HTML file
        if self.path == '/' or self.path == '/index.html':
            self.send_response(200)
            self.send_header('Content-type', 'text/html')
            self.end_headers()
            with open('public/test-overlay.html', 'rb') as f:
                self.wfile.write(f.read())
        else:
            super().do_GET()

if __name__ == '__main__':
    os.chdir('.')
    server = HTTPServer(('localhost', 8888), CORSRequestHandler)
    print('🌐 Test backend running on http://localhost:8888')
    server.serve_forever()
EOF

chmod +x /tmp/netgoat_test_backend.py

# Instructions
echo ""
echo "✅ Setup complete!"
echo ""
echo "📋 Next steps:"
echo "=============="
echo ""
echo "1️⃣  Start the test backend (in a new terminal):"
echo "   python3 /tmp/netgoat_test_backend.py"
echo ""
echo "2️⃣  Start the NetGoat agent (in another terminal):"
echo "   ./agent"
echo ""
echo "3️⃣  Add a test route using the agent's database:"
echo "   sqlite3 database/proxy.db"
echo "   INSERT INTO routes (route_type, domain, target_url, active) VALUES ('domain', 'localhost:8080', 'http://localhost:8888', 1);"
echo "   .exit"
echo ""
echo "4️⃣  Open your browser and visit:"
echo "   http://localhost:8080/test-overlay.html"
echo ""
echo "5️⃣  Look for the 🐐 NetGoat AI button in the bottom-right corner!"
echo ""
echo "💡 Tips:"
echo "   - Click the goat button to expand/collapse the debug panel"
echo "   - Try the test buttons on the page to simulate different risk levels"
echo "   - Use X-GoatAI-Features header to test AI analysis"
echo ""
echo "🧪 Test AI with curl:"
echo "   # Safe request"
echo "   curl -H 'X-GoatAI-Features: 0.1,0.2,0.05,0.3,10,200,5,0' http://localhost:8080/"
echo ""
echo "   # High-risk request (may be blocked)"
echo "   curl -H 'X-GoatAI-Features: 0.9,0.95,0.88,0.92,100,999,50,1' http://localhost:8080/"
echo ""
echo "📚 For more info, read: agent/DEBUG_OVERLAY.md"
echo ""
