const fs = require("fs");
const path = require("path");
const express = require("express");
const { Schema, model: makeModel } = require("./odm");
const app = express.Router();

function loadModels(dir = path.join(__dirname, "models")) {
  const models = new Map();
  if (!fs.existsSync(dir)) return models;
  for (const f of fs.readdirSync(dir)) {
    if (!f.endsWith(".js")) continue;
    const mod = require(path.join(dir, f));
    if (!mod.name || !mod.schema) continue;
    const m = makeModel(mod.name, mod.schema);
    models.set(mod.name, m);
  }
  return models;
}

const modelsDir = require("path").join(__dirname, "models");

function publish(evt) {
  audit.write({ user: "system", op: `odm.${evt.type}`, meta: evt });
  const msg = JSON.stringify({ type: "odm_event", evt });
  for (const ws of wssClients)
    if (ws.readyState === WebSocket.OPEN) ws.send(msg);
  for (const res of sseClients) res.write(`data: ${msg}\n\n`);
}

const models = loadModels(modelsDir);

function matchQuery(doc, q) {
  if (!q || Object.keys(q).length === 0) return true;
  for (const [k, v] of Object.entries(q)) {
    if (typeof v === "object" && v !== null) {
      if (v.$gt !== undefined && !(doc[k] > v.$gt)) return false;
      if (v.$lt !== undefined && !(doc[k] < v.$lt)) return false;
      if (v.$in !== undefined && !Array.isArray(v.$in)) return false;
      if (v.$in !== undefined && !v.$in.includes(doc[k])) return false;
      if (v.$regex !== undefined && !new RegExp(v.$regex).test(doc[k]))
        return false;
    } else {
      if (doc[k] !== v) return false;
    }
  }
  return true;
}

app.get("/models", (req, res) => {
  res.json({ models: Array.from(models.keys()) });
});

app.post("/:model/create", (req, res) => {
  const m = models.get(req.params.model);
  if (!m) return res.status(404).json({ error: "model not found" });
  try {
    const doc = m.create(req.body);
    publish({ type: "create", model: req.params.model, doc });
    res.json(doc);
  } catch (e) {
    res.status(400).json({ error: e.message });
  }
});

app.post("/:model/find", (req, res) => {
  const m = models.get(req.params.model);
  if (!m) return res.status(404).json({ error: "model not found" });
  const q = req.body || {};
  const limit = q.$limit || 100;
  const skip = q.$skip || 0;
  const pure = { ...q };
  delete pure.$limit;
  delete pure.$skip;
  const all = m.find({});
  const matched = all.filter((d) => matchQuery(d, pure));
  publish({
    type: "find",
    model: req.params.model,
    query: pure,
    count: matched.length,
  });
  res.json(matched.slice(skip, skip + limit));
});

app.get("/:model/:id", (req, res) => {
  const m = models.get(req.params.model);
  if (!m) return res.status(404).json({ error: "model not found" });
  const d = m.findById(req.params.id);
  if (!d) return res.status(404).json({ error: "not found" });
  res.json(d);
});

app.post("/:model/:id/update", (req, res) => {
  const m = models.get(req.params.model);
  if (!m) return res.status(404).json({ error: "model not found" });
  try {
    const u = m.updateById(req.params.id, req.body);
    if (!u) return res.status(404).json({ error: "not found" });
    publish({
      type: "update",
      model: req.params.model,
      id: req.params.id,
      patch: req.body,
      doc: u,
    });
    res.json(u);
  } catch (e) {
    res.status(400).json({ error: e.message });
  }
});

app.delete("/:model/:id", (req, res) => {
  const m = models.get(req.params.model);
  if (!m) return res.status(404).json({ error: "model not found" });
  const ok = !!m.deleteById(req.params.id);
  if (ok)
    publish({ type: "delete", model: req.params.model, id: req.params.id });
  res.json({ ok });
})

module.exports = app
