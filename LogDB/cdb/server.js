const express = require("express");
const http = require("http");
const WebSocket = require("ws");
const TransactionManager = require("./transaction");
const RBAC = require("./rbac");
const Audit = require("./audit");
const VectorIndex = require("./vector");
const Backup = require("./backup");
const { registerODM } = require("./odm_service");
const Auth = require("./auth");
const bodyParser = require("body-parser");
const odm = require("./odm_service")
function createServer({ port = 3000 } = {}) {
  const app = express();

  // parse JSON bodies
  app.use(bodyParser.json());

  // parse URL-encoded bodies (optional)
  app.use(bodyParser.urlencoded({ extended: true }));

  const tm = new TransactionManager();
  const rbac = new RBAC();
  const auth = new Auth(rbac);
  const audit = new Audit();
  const vindex = new VectorIndex();

  const wssClients = new Set();
  const sseClients = new Set();

  function getUserFromHeaders(headers) {
    return (headers && (headers["x-user"] || headers["X-User"])) || "anonymous";
  }

  function checkOp(op, req) {
    const authHeader =
      req.headers["authorization"] || req.headers["Authorization"];
    let user = getUserFromHeaders(req.headers);
    if (authHeader?.startsWith("Bearer ")) {
      const token = authHeader.slice(7);
      const v = auth.verify(token);
      if (v.ok) user = v.user;
      else return { ok: false, user: null };
    }
    if (!rbac.check(user, op)) return { ok: false, user };
    return { ok: true, user };
  }

  app.post("/atomic", (req, res) => {
    const chk = checkOp("insert", req);
    if (!chk.ok) return res.status(403).json({ error: "forbidden" });
    const out = tm.runAtomic(req.body.steps);
    audit.write({
      user: chk.user,
      op: "atomic",
      ok: out.ok,
      meta: { steps: req.body.steps },
    });
    res.json(out);
  });

  app.get("/find/:store", (req, res) => {
    const chk = checkOp("read", req);
    if (!chk.ok) return res.status(403).json({ error: "forbidden" });
    const store = tm.getStore(req.params.store);
    res.json(store.find(req.query || {}));
  });

  app.post("/vector/upsert", (req, res) => {
    const chk = checkOp("insert", req);
    if (!chk.ok) return res.status(403).json({ error: "forbidden" });
    const { id, vector } = req.body;
    if (!id || !Array.isArray(vector))
      return res.status(400).json({ error: "id & vector required" });
    vindex.upsert(id, vector);
    audit.write({ user: chk.user, op: "vector.upsert", id });
    res.json({ ok: true });
  });

  app.post("/backup", (req, res) => {
    const chk = checkOp("insert", req);
    if (!chk.ok) return res.status(403).json({ error: "forbidden" });
    const folder = Backup.createBackup(req.body?.name);
    audit.write({ user: chk.user, op: "backup", folder });
    res.json({ ok: true, folder });
  });

  app.post("/users", (req, res) => {
    const chk = checkOp("insert", req);
    if (!chk.ok) return res.status(403).json({ error: "forbidden" });
    try {
      rbac.addUser(req.body.username, req.body.role);
      audit.write({
        user: getUserFromHeaders(req.headers),
        op: "user.add",
        username: req.body.username,
        role: req.body.role,
      });
      res.json({ ok: true });
    } catch (e) {
      res.status(400).json({ error: e.message });
    }
  });

  app.post("/vector/search", (req, res) => {
    const chk = checkOp("read", req);
    if (!chk.ok) return res.status(403).json({ error: "forbidden" });
    if (!Array.isArray(req.body.vector))
      return res.status(400).json({ error: "vector required" });
    const out = vindex.query(req.body.vector, req.body.k || 5);
    res.json(out);
  });

  app.post("/login", (req, res) => {
    const out = auth.login(req.body.username, req.body.password);
    if (!out.ok) return res.status(401).json({ error: out.error });
    res.json({ token: out.token });
  });

  app.use("/odm", odm)

  const server = http.createServer(app);
  const wss = new WebSocket.Server({ server });

  // WS
  wss.on("connection", (ws) => {
    wssClients.add(ws);
    ws.send(JSON.stringify({ type: "welcome", tail: audit.last(50) }));
    ws.on("close", () => wssClients.delete(ws));
  });

  // SSE
  app.get("/stream/commits", (req, res) => {
    res.setHeader("Content-Type", "text/event-stream");
    res.setHeader("Cache-Control", "no-cache");
    res.setHeader("Connection", "keep-alive");
    res.write(
      `data: ${JSON.stringify({ type: "welcome", tail: audit.last(50) })}\n\n`
    );
    sseClients.add(res);
    req.on("close", () => sseClients.delete(res));
  });

  // broadcast commits
  tm.on("committed", (evt) => {
    const msg = JSON.stringify({ type: "committed", evt });
    for (const ws of wssClients)
      if (ws.readyState === WebSocket.OPEN) ws.send(msg);
    for (const res of sseClients) res.write(`data: ${msg}\n\n`);
  });

  server.listen(port, () =>
    console.log(`Server running on http://localhost:${port}`)
  );
  return { app, server, tm, rbac, audit, vindex, wss };
}

module.exports = { createServer };
