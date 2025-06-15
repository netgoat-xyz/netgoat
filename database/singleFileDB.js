import { appendFile, readFile, rename, stat } from "node:fs/promises";

export default class SingleFileDocDB {
  constructor(filePath) {
    this.filePath = filePath;
    this.walPath = filePath + ".wal";
    this.data = new Map();
    this.writeQueue = Promise.resolve();
    this.compacting = false;
  }

  async init() {
    try {
      const raw = await readFile(this.filePath, "utf8");
      if (raw) {
        const docs = JSON.parse(raw);
        for (const [coll, items] of Object.entries(docs)) {
          this.data.set(coll, new Map(items.map(doc => [doc._id, doc])));
        }
      }
    } catch {}
    await this.replayWAL();
  }

  async replayWAL() {
    try {
      const raw = await readFile(this.walPath, "utf8");
      if (!raw) return;
      const lines = raw.trim().split("\n");
      for (const line of lines) {
        const { collection, doc } = JSON.parse(line);
        this.applyDoc(collection, doc);
      }
    } catch {}
  }

  applyDoc(collection, doc) {
    if (!this.data.has(collection)) this.data.set(collection, new Map());
    this.data.get(collection).set(doc._id, doc);
  }

  async write(collection, doc) {
    if (!doc._id) throw new Error("_id required");
    const entry = JSON.stringify({ collection, doc }) + "\n";
    this.writeQueue = this.writeQueue.then(async () => {
      await appendFile(this.walPath, entry);
      this.applyDoc(collection, doc);
      if (!this.compacting) this.compactIfNeeded();
    });
    return this.writeQueue;
  }

  async read(collection, id) {
    const coll = this.data.get(collection);
    if (!coll) return null;
    return coll.get(id) || null;
  }

  async find(collection, filterFn = () => true) {
    const coll = this.data.get(collection);
    if (!coll) return [];
    return Array.from(coll.values()).filter(filterFn);
  }

  async compactIfNeeded() {
    if (this.compacting) return;
    try {
      const stats = await stat(this.walPath);
      if (!stats || stats.size < 1024 * 10) return;
    } catch {
      return;
    }
    this.compacting = true;
    try {
      const fullDump = {};
      for (const [coll, map] of this.data.entries()) {
        fullDump[coll] = Array.from(map.values());
      }
      const tmpPath = this.filePath + ".tmp";
      await this.atomicWrite(tmpPath, JSON.stringify(fullDump));
      await rename(tmpPath, this.filePath);
      await this.atomicWrite(this.walPath, "");
    } finally {
      this.compacting = false;
    }
  }

  async atomicWrite(path, data) {
    await appendFile(path + ".write", data);
    await rename(path + ".write", path);
  }
}
