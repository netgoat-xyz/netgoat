// Simple transaction manager that executes steps atomically (all-or-nothing)
const Storage = require('./storage');
const EventEmitter = require('events');

class TransactionManager extends EventEmitter {
  constructor() {
    super();
    this.stores = new Map();
  }

  getStore(name) {
    if (!this.stores.has(name)) this.stores.set(name, new Storage(name));
    return this.stores.get(name);
  }

  // steps: [{store, op: 'insert'|'update'|'delete'|'read', args: [...]}, ...]
  runAtomic(steps) {
    // Strategy: take snapshot copies, apply to snapshots, if all succeed persist real stores
    const snapshots = new Map();

    function cloneMap(map) {
      return new Map(Array.from(map.entries()).map(([k, v]) => [k, Object.assign({}, v)]));
    }

    // Prepare snapshots
    for (const s of steps) {
      const store = this.getStore(s.store);
      if (!snapshots.has(s.store)) snapshots.set(s.store, cloneMap(store.cache));
    }

    const results = [];
    try {
      for (const s of steps) {
        const store = this.getStore(s.store);
        const snap = snapshots.get(s.store);
        switch (s.op) {
          case 'insert': {
            const doc = Object.assign({}, s.args[0]);
            const id = doc._id || store._genId();
            doc._id = id;
            snap.set(id, doc);
            results.push({ ok: true, id, doc });
            break;
          }
          case 'update': {
            const [id, patch] = s.args;
            const existing = snap.get(id);
            if (!existing) throw new Error(`NotFound:${s.store}:${id}`);
            const updated = Object.assign({}, existing, patch);
            snap.set(id, updated);
            results.push({ ok: true, id, doc: updated });
            break;
          }
          case 'delete': {
            const [id] = s.args;
            const ok = snap.delete(id);
            if (!ok) throw new Error(`NotFound:${s.store}:${id}`);
            results.push({ ok: true, id });
            break;
          }
          case 'read': {
            const [query] = s.args;
            const out = [];
            for (const d of snap.values()) {
              let ok = true;
              for (const k of Object.keys(query || {})) if (d[k] !== query[k]) { ok = false; break; }
              if (ok) out.push(d);
            }
            results.push({ ok: true, out });
            break;
          }
          default:
            throw new Error('UnknownOp:' + s.op);
        }
      }

      // All steps succeeded on snapshots; commit to real stores
      for (const [storeName, snap] of snapshots.entries()) {
        const store = this.getStore(storeName);
        store.cache = snap;
        store._persist();
      }

      const res = { ok: true, results };
      // emit event for listeners (e.g., WebSocket broadcasters, audit)
      this.emit('committed', { steps, results });
      return res;
    } catch (err) {
      const res = { ok: false, error: err.message, results };
      this.emit('failed', res);
      return res;
    }
  }
}

module.exports = TransactionManager;
