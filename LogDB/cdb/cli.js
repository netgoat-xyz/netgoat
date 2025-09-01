const { Command } = require('commander');
const { createApp } = require('./api');
const server = require('http');

const program = new Command();
program.version('0.1.0');

program.command('serve [port]').description('Start HTTP API').action((port = 3000) => {
  const { app } = createApp();
  const s = server.createServer(app);
  s.listen(port, () => console.log(`API listening on http://localhost:${port}`));
});

// parse will be called after commands are defined

// REPL command: starts a node REPL with helper bindings (tm, model, models, Storage)
program.command('repl').description('Start interactive REPL with DB bindings').action(async () => {
  const repl = require('repl')
  const fs = require('fs')
  const path = require('path')
  const Storage = require('./storage')
  const { createApp } = require('./api')
  const { Schema, model, Types } = require('./odm')

  const appContext = createApp()
  const tm = appContext.tm

  // load models folder (all exports merged)
  const models = {}
  const modelsDir = path.join(__dirname, 'models')
  if (fs.existsSync(modelsDir)) {
    for (const f of fs.readdirSync(modelsDir)) {
      if (!/\.js$/.test(f)) continue
      try {
        const mod = require(path.join(modelsDir, f))
        if (mod && typeof mod === 'object') Object.assign(models, mod)
      } catch (e) {
        console.warn('Failed to load model', f, e && e.message)
      }
    }
  }

  // db helper for REPL
  const db = {
    tm,
    dataDir: path.join(__dirname, '..', 'data'),
    memory() {
      const mu = process.memoryUsage()
      const stats = { rss: mu.rss, heapTotal: mu.heapTotal, heapUsed: mu.heapUsed, external: mu.external, arrayBuffers: mu.arrayBuffers }
      // data dir size
      let dataBytes = 0
      try {
        if (fs.existsSync(this.dataDir)) {
          for (const f of fs.readdirSync(this.dataDir)) {
            const p = path.join(this.dataDir, f)
            const st = fs.statSync(p)
            dataBytes += st.size
          }
        }
      } catch (e) {}
      stats.dataBytes = dataBytes
      // per-collection doc counts
      const counts = {}
      try { for (const [name, s] of tm.stores.entries()) counts[name] = s.cache.size } catch(e) {}
      stats.counts = counts
      return stats
    },
    collections() {
      const fromTm = Array.from(tm.stores.keys())
      let fromFs = []
      try { fromFs = fs.readdirSync(this.dataDir).map(f=>f.replace(/\.bson$|\.json$/,'')) } catch (_) {}
      return Array.from(new Set([...fromTm, ...fromFs]))
    },
    collection(name) {
      const store = tm.getStore(name)
      return {
        find: (q) => store.find(q),
        insert: (doc) => store.insert(doc),
        update: (id, patch) => store.update(id, patch),
        delete: (id) => store.delete(id),
        raw: store,
        persist: () => { store._persist(); return true },
  persistSafe: () => { try { store._persist(); return { ok:true } } catch (e) { return { ok:false, error: e && e.message } } },
        reload: () => { tm.stores.set(name, new Storage(name)); return tm.getStore(name) }
      }
    },
    getStore: (name) => tm.getStore(name),
    atomic: (steps) => tm.runAtomic(steps),
    saveAll: () => { for (const s of tm.stores.values()) s._persist(); return true },
    backup: (name) => {
      // simple backup: copy file to .bak
      try {
        const file = path.join(this.dataDir, name + '.bson')
        if (!fs.existsSync(file)) throw new Error('no file')
        const dst = file + '.bak-' + Date.now()
        fs.copyFileSync(file, dst)
        return dst
      } catch (e) { throw e }
    }
  }

  // convenience: insert many documents into a collection with one atomic call when possible
  // usage: await db.insertManyAtomic('test', docs, { batch: 1000 })
  db.insertManyAtomic = function(collection, docs, opts = {}) {
    const batch = opts.batch || 2000
    const chunked = opts.chunked || false
    if (!Array.isArray(docs)) throw new Error('docs must be an array')

    // if chunked explicitly requested, or docs length > batch, do chunked commits
    if (chunked || docs.length > batch) {
      const out = []
      for (let i = 0; i < docs.length; i += batch) {
        const chunk = docs.slice(i, i + batch)
        const steps = chunk.map(d => ({ store: collection, op: 'insert', args: [d] }))
        const r = db.atomic(steps)
        out.push(r)
        if (!r.ok) break
      }
      return out
    }

    // single atomic commit
    const steps = docs.map(d => ({ store: collection, op: 'insert', args: [d] }))
    return db.atomic(steps)
  }

  const serverRepl = repl.start({ prompt: 'db> ' })
  serverRepl.context.tm = tm
  serverRepl.context.model = model
  serverRepl.context.Schema = Schema
  serverRepl.context.Types = Types
  serverRepl.context.models = models
  serverRepl.context.db = db
  try {
    const ingest = require('./ingest')
    serverRepl.context.ingest = ingest
    serverRepl.context.db.ingestMetrics = () => ingest.metrics()
  } catch (_) {}
  serverRepl.context.reload = () => {
    // reload all models
    try {
      for (const f of fs.readdirSync(modelsDir).filter(x=>/\.js$/.test(x))) {
        delete require.cache[require.resolve(path.join(modelsDir, f))]
      }
      for (const f of fs.readdirSync(modelsDir).filter(x=>/\.js$/.test(x))) {
        try { const m = require(path.join(modelsDir, f)); Object.assign(models, m) } catch(e){ console.warn('reload model failed', f, e && e.message) }
      }
      return models
    } catch (e) { throw e }
  }
  serverRepl.context.quit = serverRepl.context.exit = () => process.exit(0)
})


// health command: prints memory, data size, and a quick estimate for 1M docs feasibility
program.command('health').description('Show DB memory and storage health').action(() => {
  const { tm } = createApp()
  const fs = require('fs')
  const path = require('path')
  const dataDir = path.join(__dirname, '..', 'data')
  const mu = process.memoryUsage()
  let dataBytes = 0
  try { if (fs.existsSync(dataDir)) { for (const f of fs.readdirSync(dataDir)) { dataBytes += fs.statSync(path.join(dataDir, f)).size } } } catch(e){}

  console.log('memoryUsage:', { rss: mu.rss, heapUsed: mu.heapUsed, heapTotal: mu.heapTotal })
  console.log('dataDirBytes:', dataBytes)

  // conservative estimate: sample one collection size on disk and extrapolate
  const files = fs.existsSync(dataDir) ? fs.readdirSync(dataDir).filter(x=>/\.bson$|\.json$/.test(x)) : []
  if (files.length) {
    const sample = files[0]
    const size = fs.statSync(path.join(dataDir, sample)).size
    console.log('sample file:', sample, 'sizeBytes:', size)
    console.log('estimated bytes for 1M docs (if sample contains 1000 docs):', Math.round((size / 1000) * 1000000))
  }

  console.log('loadedCollections:', Array.from(tm.stores.keys()))
  process.exit(0)
})
program.parse(process.argv);
