import WebSocket, { WebSocketServer } from "ws";
import readline from "readline";
import fs from "fs";

const LOG_FILE = "./clients.json";
let clients = [];

// Load existing clients from file if any
try {
  clients = JSON.parse(fs.readFileSync(LOG_FILE));
} catch {
  clients = [];
}

const wss = new WebSocketServer({ port: 8080 });

function saveClients() {
  fs.writeFileSync(LOG_FILE, JSON.stringify(clients, null, 2));
}

wss.on("connection", (ws, req) => {
  // Real IP from Cloudflare or fallback
  const ip =
    req.headers["cf-connecting-ip"] ||
    req.headers["x-forwarded-for"]?.split(",")[0].trim() ||
    req.socket.remoteAddress ||
    "unknown";

  // Create client record
  const clientId = Date.now() + Math.random().toString(36).slice(2, 8);
  const clientInfo = {
    id: clientId,
    ip,
    connectedAt: new Date().toISOString(),
    data: null, // will hold client data sent from client
  };
  clients.push(clientInfo);
  saveClients();

  console.log(`Client connected: ${ip}`);

  ws.on("message", (msg) => {
    const message = msg.toString();
    console.log(`Client (${ip}) says:`, message);

    try {
      const parsed = JSON.parse(message);
      if (parsed.type === "clientInfo") {
        // Update client's data
        const idx = clients.findIndex((c) => c.id === clientId);
        if (idx !== -1) {
          clients[idx].data = parsed.data;
          saveClients();
        }
      }
    } catch {
      // Not JSON, ignore or handle other messages here
    }
  });

  ws.on("close", () => {
    clients = clients.filter((c) => c.id !== clientId);
    saveClients();
    console.log(`Client disconnected: ${ip}`);
  });
});

// REPL CLI for sending scripts
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  prompt: "send-script> ",
});

rl.prompt();

rl.on("line", (line) => {
  if (!line.trim()) {
    rl.prompt();
    return;
  }

  wss.clients.forEach((ws) => {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(line);
    }
  });

  console.log(`Sent to ${wss.clients.size} client(s)`);
  rl.prompt();
}).on("close", () => {
  console.log("REPL closed, shutting down server...");
  wss.close();
  process.exit(0);
});
