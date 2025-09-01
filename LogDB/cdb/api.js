const express = require('express');
const bodyParser = require('body-parser');
const TransactionManager = require('./transaction');
const ingest = require('./ingest');

function createApp() {
  const app = express();
  app.use(bodyParser.json());
  const tm = new TransactionManager();
  // init ingest worker
  ingest.init(tm, { batchSize: 1000, flushInterval: 250 })

  app.post('/atomic', (req, res) => {
    const { steps } = req.body;
    if (!Array.isArray(steps)) return res.status(400).json({ error: 'steps array required' });
    const out = tm.runAtomic(steps);
    if (!out.ok) return res.status(500).json(out);
    res.json(out);
  });

  app.get('/find/:store', (req, res) => {
    const store = tm.getStore(req.params.store);
    const q = req.query || {};
    res.json(store.find(q));
  });

  app.post('/ingest', (req, res) => {
    const doc = req.body
    if (!doc) return res.status(400).json({ error: 'body required' })
    const out = ingest.enqueue(doc)
    if (!out.ok) return res.status(503).json(out)
    res.json({ ok: true })
  })

  return { app, tm };
}

module.exports = { createApp };
