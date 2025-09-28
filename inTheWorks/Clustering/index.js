import express from "express";
import mongoose from "mongoose";
import os from "os";
import crypto from "crypto";
import WebSocket from "ws";

// ===== CONFIG =====
const MONGO_URI = process.env.MONGO_URI || "mongodb://localhost:27017/cluster";
const PORT = process.env.PORT || Math.floor(Math.random() * (65535 - 1024) + 1024);
const P2P_PORT = process.env.P2P_PORT || Math.floor(Math.random() * (65535 - 1024) + 1024);
// ==================

await mongoose.connect(MONGO_URI, { autoIndex: true });

const nodeSchema = new mongoose.Schema({
  nodeId: { type: String, unique: true },
  host: String,
  port: Number,
  cpuLoad: Number,
  mem: Object,
  ts: { type: Date, default: Date.now },
});
nodeSchema.index({ ts: 1 }, { expireAfterSeconds: 60 });
const Node = mongoose.model("Node", nodeSchema);

const clusterSchema = new mongoose.Schema({
  _id: { type: String, default: "cluster" },
  leaderId: String,
  updatedAt: { type: Date, default: Date.now },
});
const ClusterState = mongoose.model("ClusterState", clusterSchema);

const app = express();
const server = app.listen(PORT, async () => {
  const actualPort = server.address().port;
  const nodeId = `${os.hostname()}-${actualPort}-${crypto.randomUUID()}`;
  console.log(`[DEBUG] Node online → ${nodeId} @ :${actualPort}`);

  // ---------- heartbeat ----------
  async function heartbeat() {
    const cpu = os.loadavg()[0] / os.cpus().length;
    const mem = process.memoryUsage();

    await Node.findOneAndUpdate(
      { nodeId },
      { nodeId, host: os.hostname(), port: actualPort, cpuLoad: cpu, mem, ts: new Date() },
      { upsert: true, new: true }
    );

    // leader election
    const best = await Node.find().sort({ cpuLoad: 1 }).limit(1);
    if (best.length) {
      await ClusterState.findByIdAndUpdate(
        "cluster",
        { leaderId: best[0].nodeId, updatedAt: new Date() },
        { upsert: true }
      );
    }
  }

  await heartbeat();
  setInterval(heartbeat, 10_000);

  // ---------- P2P ----------
  const peers = new Set(); // list of ws connections

  const wss = new WebSocket.Server({ port: P2P_PORT });
  console.log(`[DEBUG] P2P listening on :${P2P_PORT}`);

  wss.on("connection", (ws, req) => {
    console.log("[DEBUG] P2P connection from", req.socket.remoteAddress);
    peers.add(ws);

    ws.on("message", async (msg) => {
      try {
        const data = JSON.parse(msg);
        if (data.type === "node_update") {
          // upsert peer node
          await Node.findOneAndUpdate(
            { nodeId: data.node.nodeId },
            data.node,
            { upsert: true }
          );
        }
      } catch (e) {
        console.error("[ERROR] P2P message parse failed", e.message);
      }
    });

    ws.on("close", () => peers.delete(ws));
  });

  // Gossip heartbeat to peers
  setInterval(async () => {
    const node = await Node.findOne({ nodeId }).lean();
    for (const peer of peers) {
      if (peer.readyState === WebSocket.OPEN) {
        peer.send(JSON.stringify({ type: "node_update", node }));
      }
    }
  }, 5_000);

  // ---------- routes ----------
  app.get("/nodes", async (_req, res) => res.json(await Node.find().lean()));
  app.get("/leader", async (_req, res) => {
    const cluster = await ClusterState.findById("cluster").lean();
    res.json({ leader: cluster?.leaderId || null });
  });
});
