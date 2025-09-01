const fs = require('fs');
const path = require('path');
let BSON;
try { BSON = require('bson'); } catch (e) { BSON = null; }

const DB_DIR = path.join(__dirname, '..', 'data');
if (!fs.existsSync(DB_DIR)) fs.mkdirSync(DB_DIR, { recursive: true });

// shard settings
const SHARD_DOC_LIMIT = 20000; // max documents per shard file before auto-sharding
const SHARD_NAME = (collection, idx) => `${collection}.shard-${String(idx).padStart(3,'0')}.bson`;
const SHARD_JSONL_GZ = (collection, idx) => `${collection}.shard-${String(idx).padStart(3,'0')}.jsonl.gz`;
const SHARD_JSONL = (collection, idx) => `${collection}.shard-${String(idx).padStart(3,'0')}.jsonl`;

class Storage {
  constructor(collection) {
    this.collection = collection;
    this.file = path.join(DB_DIR, `${collection}.bson`);
    this.cache = new Map();
    this._load();
  }

  async _load() {
    // load all shard/bson/json files for this collection; stream large JSONL shards to avoid memory spikes
    try {
      const files = fs.readdirSync(DB_DIR).filter(f => f.startsWith(this.collection + '.'));
      const docs = [];
      for (const f of files) {
        const p = path.join(DB_DIR, f);
        if (!fs.existsSync(p)) continue;
        // handle gzip jsonl
        if (f.endsWith('.jsonl.gz')) {
          try {
            const zlib = require('zlib')
            const readline = require('readline')
            const stream = fs.createReadStream(p).pipe(zlib.createGunzip())
            const rl = readline.createInterface({ input: stream, crlfDelay: Infinity })
            for await (const line of rl) {
              if (!line || !line.trim()) continue
              try { const obj = JSON.parse(line); docs.push(obj) } catch (_) {}
            }
            continue
          } catch (e) {
            console.error('Failed to stream-parse', p, e && e.message ? e.message : e)
            try { const corruptPath = p + `.corrupt-${Date.now()}.bak`; fs.renameSync(p, corruptPath); console.error('Moved corrupt file to', corruptPath) } catch(_){}
            continue
          }
        }

        // handle jsonl
if (f.endsWith('.jsonl')) {
  try {
    const readline = require('readline')
    const stream = fs.createReadStream(p)
    const rl = readline.createInterface({ input: stream, crlfDelay: Infinity })

    async function parseLines() {
      for await (const line of rl) {
        if (!line || !line.trim()) continue
        try { 
          const obj = JSON.parse(line)
          docs.push(obj)
        } catch (_) {}
      }
    }

    await parseLines()
    continue
  } catch (e) {
    console.error('Failed to stream-parse', p, e?.message ?? e)
    try { 
      const corruptPath = p + `.corrupt-${Date.now()}.bak`
      fs.renameSync(p, corruptPath)
      console.error('Moved corrupt file to', corruptPath)
    } catch(_){}
    continue
  }
}


        const buf = fs.readFileSync(p);
        if (!buf || buf.length === 0) continue;
        // try JSON first (single-file JSON)
        const txt = buf.toString('utf8').trim();
        if (txt.startsWith('{') || txt.startsWith('[')) {
          try {
            const obj = JSON.parse(txt);
            if (obj && obj.docs) {
              for (const d of obj.docs) docs.push(d);
              continue;
            }
          } catch (je) {
            // fall through to BSON attempt
          }
        }

        if (BSON && (Buffer.isBuffer(buf) || buf instanceof Uint8Array)) {
          try {
            let arr;
            if (typeof BSON.deserialize === 'function') arr = BSON.deserialize(buf);
            else if (BSON && BSON.BSON && typeof BSON.BSON.deserialize === 'function') arr = BSON.BSON.deserialize(buf);
            else if (typeof BSON === 'function' && typeof BSON.prototype.deserialize === 'function') arr = BSON.prototype.deserialize(buf);
            if (arr && arr.docs) for (const d of arr.docs) docs.push(d);
            continue;
          } catch (be) {
            console.error('Failed to parse shard', p, be && be.message ? be.message : be);
            try { const corruptPath = p + `.corrupt-${Date.now()}.bak`; fs.renameSync(p, corruptPath); console.error('Moved corrupt file to', corruptPath) } catch(_){}
            continue;
          }
        }

        // last attempt: JSON parse of text
        try {
          const obj = JSON.parse(txt);
          if (obj && obj.docs) for (const d of obj.docs) docs.push(d);
        } catch (je) {
          console.error('Invalid file format for', p);
          try { const corruptPath = p + `.corrupt-${Date.now()}.bak`; fs.renameSync(p, corruptPath); console.error('Moved corrupt file to', corruptPath) } catch(_){}
        }
      }
      if (docs.length) this.cache = new Map(docs.map(d => [d._id, d]));
    } catch (e) {
      console.error('Failed to load collection', this.collection, e && e.message ? e.message : e);
      this.cache = new Map();
    }
  }

  async _persist() {
    const docs = Array.from(this.cache.values());
    // ensure parent directory exists (fixes ENOENT on some systems)
    const dir = path.dirname(this.file);
    if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
    // Attempt BSON serialization and auto-shard if necessary. Create shard files when docs exceed SHARD_DOC_LIMIT.
    const writeAtomic = (targetPath, content, encoding) => {
      const tmp = targetPath + '.tmp-' + Date.now();
      fs.writeFileSync(tmp, content, encoding);
      fs.renameSync(tmp, targetPath);
    }

    // helper to write a shard. For large slices we stream NDJSON (gzipped) to avoid large buffers.
  const writeShard = async (idx, docsSlice) => {
      const shardBsonPath = path.join(dir, SHARD_NAME(this.collection, idx))
      const shardJsonlGz = path.join(dir, SHARD_JSONL_GZ(this.collection, idx))
      const shardJsonl = path.join(dir, SHARD_JSONL(this.collection, idx))

      // if large slice, stream as gzipped NDJSON
      if (docsSlice.length > 2000) {
        try {
          const zlib = require('zlib')
          const gzip = zlib.createGzip({ level: 6 })
          const tmpPath = shardJsonlGz + '.tmp-' + Date.now()
          const out = fs.createWriteStream(tmpPath)
          const stream = gzip.pipe(out)
          for (const d of docsSlice) {
            const line = JSON.stringify(d) + '\n'
            if (!gzip.write(line)) {
              // backpressure: wait for drain
              await new Promise(r => gzip.once('drain', r))
            }
          }
          gzip.end()
          await new Promise((resolve, reject) => { out.on('finish', resolve); out.on('error', reject) })
          const finalPath = shardJsonlGz
          fs.renameSync(tmpPath, finalPath)
          try { if (fs.existsSync(shardBsonPath)) fs.unlinkSync(shardBsonPath) } catch(_){}
          return finalPath
        } catch (e) {
          console.error('Shard JSONL-GZ write failed for', this.collection, e && e.message ? e.message : e)
        }
      }

      // otherwise try BSON small write
      try {
        if (BSON && typeof BSON.serialize === 'function') {
          const raw = BSON.serialize({ docs: docsSlice });
          writeAtomic(shardBsonPath, raw);
          try { if (fs.existsSync(shardJsonl)) fs.unlinkSync(shardJsonl) } catch(_){}
          try { if (fs.existsSync(shardJsonlGz)) fs.unlinkSync(shardJsonlGz) } catch(_){}
          return shardBsonPath
        } else if (BSON && typeof BSON.BSON === 'function' && typeof BSON.BSON.serialize === 'function') {
          const raw = BSON.BSON.serialize({ docs: docsSlice });
          writeAtomic(shardBsonPath, raw);
          try { if (fs.existsSync(shardJsonl)) fs.unlinkSync(shardJsonl) } catch(_){}
          try { if (fs.existsSync(shardJsonlGz)) fs.unlinkSync(shardJsonlGz) } catch(_){}
          return shardBsonPath
        }
      } catch (e) {
        console.error('Shard BSON write failed for', this.collection, 'shard', idx, e && e.message ? e.message : e);
      }

      // fallback: write compact JSONL (no pretty spacing)
      try {
        const outPath = shardJsonl
        const tmp = outPath + '.tmp-' + Date.now()
        const w = fs.createWriteStream(tmp)
        for (const d of docsSlice) w.write(JSON.stringify(d) + '\n')
        w.end()
        return tmp && (fs.existsSync(tmp) ? (fs.renameSync(tmp, outPath), outPath) : outPath)
      } catch (je) {
        console.error('Shard JSONL write failed for', this.collection, je && je.message ? je.message : je)
        throw je
      }
    }

    try {
      if (docs.length <= SHARD_DOC_LIMIT) {
        // try single-file write
        try {
          if (BSON && typeof BSON.serialize === 'function') {
            const raw = BSON.serialize({ docs });
            writeAtomic(this.file, raw);
            // remove any old shard files
            const oldShards = fs.readdirSync(dir).filter(f => f.startsWith(this.collection + '.shard-'))
            for (const s of oldShards) try { fs.unlinkSync(path.join(dir, s)) } catch(_){}
            return;
          } else if (BSON && typeof BSON.BSON === 'function' && typeof BSON.BSON.serialize === 'function') {
            const raw = BSON.BSON.serialize({ docs });
            writeAtomic(this.file, raw);
            const oldShards = fs.readdirSync(dir).filter(f => f.startsWith(this.collection + '.shard-'))
            for (const s of oldShards) try { fs.unlinkSync(path.join(dir, s)) } catch(_){}
            return;
          }
        } catch (e) {
          console.error('Single-file BSON write failed, will try shard fallback', e && e.message ? e.message : e);
        }
      }

      // Write as shards
      const numShards = Math.ceil(docs.length / SHARD_DOC_LIMIT)
      const written = []
      for (let i = 0; i < numShards; i++) {
        const slice = docs.slice(i * SHARD_DOC_LIMIT, (i + 1) * SHARD_DOC_LIMIT)
        const p = await writeShard(i, slice)
        written.push(p)
      }

      // remove leftover shard files beyond written count
      const existingShards = fs.readdirSync(dir).filter(f => f.startsWith(this.collection + '.shard-'))
      existingShards.forEach(f => {
        const idx = parseInt((f.match(/shard-(\d+)/) || [])[1], 10)
        if (isNaN(idx) || idx >= numShards) {
          try { fs.unlinkSync(path.join(dir, f)) } catch(_){}
        }
      })

      // also remove single-file main .bson if present (we now use shards)
      try { if (fs.existsSync(this.file)) fs.unlinkSync(this.file) } catch(_){}

    } catch (err) {
      // last-resort: JSON fallback to a single .json file
      console.error('Persist failed with shards for', this.collection, err && err.message ? err.message : err);
      try {
        const json = JSON.stringify({ docs }, null, 2);
        const jsonPath = this.file.replace(/\.bson$/i, '.json');
        writeAtomic(jsonPath, json, 'utf8');
      } catch (je) {
        console.error('Final JSON persist failed for', this.collection, je && je.message ? je.message : je);
      }
    }
  }

  insert(doc) {
    const id = doc._id || this._genId();
    const copy = Object.assign({}, doc, { _id: id });
    this.cache.set(id, copy);
  this._persist().catch(e => console.error('persist error', e && e.message ? e.message : e));
    return copy;
  }

  find(query = {}) {
    // naive matcher: exact match on fields present in query
    const results = [];
    for (const doc of this.cache.values()) {
      let ok = true;
      for (const k of Object.keys(query)) {
        if (doc[k] !== query[k]) { ok = false; break; }
      }
      if (ok) results.push(doc);
    }
    return results;
  }

  update(id, patch) {
    const existing = this.cache.get(id);
    if (!existing) return null;
    const updated = Object.assign({}, existing, patch);
    this.cache.set(id, updated);
  this._persist().catch(e => console.error('persist error', e && e.message ? e.message : e));
    return updated;
  }

  delete(id) {
    const ok = this.cache.delete(id);
  this._persist().catch(e => console.error('persist error', e && e.message ? e.message : e));
    return ok;
  }

  _genId() {
    return Math.random().toString(36).slice(2, 10);
  }
}

module.exports = Storage;
