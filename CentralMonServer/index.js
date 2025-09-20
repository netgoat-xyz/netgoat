/* server.js
   Combined Elysia + Mongo + Fake SSH with lots of features
*/
import { Elysia, t } from "elysia";
import mongoose from "mongoose";
import jwt from "jsonwebtoken";
import chalk from "chalk";
import crypto from "crypto";
import { html } from "@elysiajs/html";
import fs from "fs";
import fsPromises from "fs/promises";
import os from "os";
import util from "util";
import { Server } from "ssh2";
import repl from "repl";
import Table from "cli-table3";
import path from "path";

// --- Config ---
const PORT = process.env.PORT || 1933;
const MONGODB_URI = process.env.MONGODB_URI || "mongodb://localhost:27017/stats_db";
const SHARED_JWT_SECRET = process.env.SHARED_JWT_SECRET || "shared_secret";
const DYNAMIC_SECRET_KEY_JWT_SECRET = process.env.DYNAMIC_SECRET_KEY_JWT_SECRET || "dynamic_secret";
const LOG_FILE = process.env.LOG_FILE || "./server.log";
const BACKUP_DIR = process.env.BACKUP_DIR || "./backups";
await fsPromises.mkdir(BACKUP_DIR, { recursive: true });

// --- Logging helper ---
const levels = {
  info: { color: chalk.cyan, emoji: "ℹ️ " },
  warn: { color: chalk.yellow, emoji: "⚠️ " },
  error: { color: chalk.red, emoji: "❌ " },
  debug: { color: chalk.magenta, emoji: "🐛 " },
  success: { color: chalk.green, emoji: "✅" },
};
const serverLog = (level, ...msg) => {
  const { color, emoji } = levels[level] || levels.info;
  const timestamp = new Date().toISOString();
  const output = `${emoji} ${timestamp} ${level.toUpperCase()} › ${msg.join(" ")}`;
  console.log(color(output));
  try { fs.appendFileSync(LOG_FILE, output + "\n"); } catch (e) {}
};

// --- Mongo connection & models ---
await mongoose.connect(MONGODB_URI).then(() => serverLog("success", `Connected to MongoDB ${MONGODB_URI}`)).catch((e) => serverLog("error", "Mongo connection failed:", e.message));

const secretKeySchema = new mongoose.Schema({
  instanceId: { type: String, unique: true },
  service: String,
  workerId: { type: String, default: "default_worker" },
  regionId: String,
  secretKey: String,
  createdAt: { type: Date, default: Date.now },
  updatedAt: { type: Date, default: Date.now },
});
const SecretKeyModel = mongoose.model("SecretKey", secretKeySchema);

const statReportSchema = new mongoose.Schema({
  dataKey: String,
  service: String,
  workerId: { type: String, default: "default_worker" },
  regionId: String,
  stats: Object,
  receivedAt: { type: Date, default: Date.now },
});
statReportSchema.index({ dataKey: 1, receivedAt: -1 });
statReportSchema.index({ service: 1, regionId: 1, receivedAt: -1 });
const StatReportModel = mongoose.model("StatReport", statReportSchema);

// --- Elysia API (kept small) ---
const app = new Elysia()
  .use(html())
  .post("/auth", async ({ body, headers, set }) => {
    try {
      console.log("Auth attempt from", headers['x-forwarded-for'] || headers.host || "unknown");
      const authHeader = headers.authorization;
      if (!authHeader || !authHeader.startsWith("Bearer ")) { set.status = 401; return { message: "Unauthorized" }; }
      jwt.verify(authHeader.split(" ")[1], SHARED_JWT_SECRET);
      const { service, workerId, regionId } = body;
      if (!service || !regionId) { set.status = 400; return { message: "Missing service/regionId" }; }
      const instanceId = workerId && workerId !== "default_worker" ? `${service}_${regionId}_${workerId}` : `${service}_${regionId}`;
      let sk = await SecretKeyModel.findOne({ instanceId });
      if (!sk) {
        sk = await SecretKeyModel.create({ instanceId, service, workerId: workerId || "default_worker", regionId, secretKey: crypto.randomUUID() });
      }
      const token = jwt.sign({ secretKey: sk.secretKey, instanceId }, DYNAMIC_SECRET_KEY_JWT_SECRET, { expiresIn: "1h" });
      console.log("Generated token for", instanceId);
      return { token };
    } catch (e) { set.status = 401; return { message: "Unauthorized" }; }
  }, { body: t.Object({ service: t.String(), workerId: t.Optional(t.String()), regionId: t.String() }) })
  .post("/report-stats", async ({ body, headers, set }) => {
    try {
      const secretKeyHeader = headers["x-secret-key"];
      if (!secretKeyHeader) { set.status = 401; return { message: "Missing X-Secret-Key" }; }
      const { dataKey, service, workerId, regionId, stats } = body;
      if (!dataKey || !service || !regionId || !stats) { set.status = 400; return { message: "Missing fields" }; }
      const instanceId = workerId && workerId !== "default_worker" ? `${service}_${regionId}_${workerId}` : `${service}_${regionId}`;
      const sk = await SecretKeyModel.findOne({ instanceId });
      if (!sk || sk.secretKey !== secretKeyHeader) { set.status = 403; return { message: "Forbidden" }; }
      await StatReportModel.create({ dataKey, service, workerId: workerId || "default_worker", regionId, stats });
      return { message: "Stored" };
    } catch (e) { set.status = 500; return { message: "Error" }; }
  })
  .get("/api/stats", async ({ query }) => {
    const { service, region, limit = 20 } = query;
    const filter = {};
    if (service) filter.service = service;
    if (region) filter.regionId = region;
    const reports = await StatReportModel.find(filter).sort({ receivedAt: -1 }).limit(Number(limit)).lean();
    return { reports };
  })
  .listen(PORT, () => serverLog("success", `ElysiaJS running at http://localhost:${PORT}`));

// --- Fake shell infrastructure ---
const username = "ducky";
const password = "quack";
const hostKeyPath = "host_ed25519";
if (!fs.existsSync(hostKeyPath)) { // generate a dummy host key if missing (ED25519)
  serverLog("warn", "host_ed25519 not found — create one with ssh-keygen or place a key file.");
}
const hostKey = fs.existsSync(hostKeyPath) ? fs.readFileSync(hostKeyPath) : Buffer.from(crypto.randomBytes(64));

// in-memory fake FS and process table
let fakeFS = { "/": {} };
let cwd = "/";
let processes = [{ pid: 1, name: "init", startedAt: Date.now() }];
let nextPid = 2;
let transactionBuffer = [];

// utility helpers
const resolvePath = (p) => p.startsWith("/") ? path.normalize(p) : path.normalize(path.join(cwd, p));
const prettyJSON = (v) => util.inspect(v, { colors: true, depth: 5 });

// --- Feature Implementations ---
// Backup / restore:
async function backupAllCollections(tag = null) {
  const time = new Date().toISOString().replace(/[:.]/g, "-");
  const name = `${tag || "dump"}-${time}.json`;
  const outPath = path.join(BACKUP_DIR, name);
  const all = {};
  const models = [{ name: "SecretKey", model: SecretKeyModel }, { name: "StatReport", model: StatReportModel }];
  for (const m of models) {
    all[m.name] = await m.model.find({}).lean();
  }
  await fsPromises.writeFile(outPath, JSON.stringify(all, null, 2), "utf8");
  return outPath;
}
async function restoreFromBackup(filePath, { drop = false } = {}) {
  const content = await fsPromises.readFile(filePath, "utf8");
  const parsed = JSON.parse(content);
  if (parsed.SecretKey) {
    if (drop) await SecretKeyModel.deleteMany({});
    await SecretKeyModel.insertMany(parsed.SecretKey);
  }
  if (parsed.StatReport) {
    if (drop) await StatReportModel.deleteMany({});
    await StatReportModel.insertMany(parsed.StatReport);
  }
  return true;
}
async function exportCollection(name) {
  const model = getModel(name);
  const docs = await model.find({}).lean();
  return JSON.stringify(docs, null, 2);
}
async function importCollection(name, jsonText) {
  const docs = JSON.parse(jsonText);
  const model = getModel(name);
  await model.insertMany(docs);
  return docs.length;
}
async function aggregateCollection(name, pipelineJson) {
  const model = getModel(name);
  const pipeline = typeof pipelineJson === "string" ? JSON.parse(pipelineJson) : pipelineJson;
  return model.aggregate(pipeline).limit(50);
}
async function rotateSecretKey(instanceId) {
  const newKey = crypto.randomUUID();
  const doc = await SecretKeyModel.findOneAndUpdate({ instanceId }, { secretKey: newKey, updatedAt: new Date() }, { new: true });
  return doc;
}
function generateInstanceJWT(instanceId, expires = "1h") {
  return jwt.sign({ instanceId }, DYNAMIC_SECRET_KEY_JWT_SECRET, { expiresIn: expires });
}
async function simulateLoad(instanceId, cpuPercent = 50, durationMs = 5000, freqMs = 1000) {
  const [service, region, worker] = instanceId.split("_");
  const steps = Math.ceil(durationMs / freqMs);
  const created = [];
  for (let i = 0; i < steps; i++) {
    const stats = {
      timestamp: new Date(),
      hostname: `${service}-${worker || "w"}`,
      platform: os.platform(),
      arch: os.arch(),
      systemUptimeSeconds: process.uptime(),
      cpuCount: os.cpus().length,
      cpuModel: os.cpus()[0].model,
      systemLoadAverage: os.loadavg(),
      totalSystemMemoryBytes: os.totalmem(),
      freeSystemMemoryBytes: os.freemem(),
      usedSystemMemoryBytes: os.totalmem() - os.freemem(),
      systemMemoryUsagePercent: ((os.totalmem() - os.freemem())/os.totalmem())*100,
      processId: process.pid,
      processUptimeSeconds: process.uptime(),
      appCpuUsagePercent: cpuPercent,
      appMemoryRssBytes: process.memoryUsage().rss,
    };
    const dataKey = `${service || "svc"}_${region || "reg"}_${worker || "w"}`;
    created.push(await StatReportModel.create({ dataKey, service: service || "svc", workerId: worker || "default_worker", regionId: region || "reg", stats }));
    await new Promise(res => setTimeout(res, freqMs));
  }
  return created.length;
}
async function pruneReports(olderThanDays = 30) {
  const cutoff = new Date(Date.now() - olderThanDays * 24 * 60 * 60 * 1000);
  const res = await StatReportModel.deleteMany({ receivedAt: { $lt: cutoff } });
  return res.deletedCount;
}
async function healthCheck() {
  const mongoOk = mongoose.connection.readyState === 1;
  const disk = (await fsPromises.stat(".")).size || 0;
  return { mongoConnected: mongoOk, uptimeSec: process.uptime(), diskSampleBytes: disk };
}

// runner helpers
async function runSQLLike(query) {
  // small SQL-ish parser (supports SELECT, INSERT, DELETE with basic WHERE =field='value')
  const q = query.trim();
  const verb = q.split(/\s+/)[0]?.toUpperCase();
  if (verb === "SELECT") {
    // SELECT * FROM StatReport WHERE service='api'
    const m = (q.match(/FROM\s+(\w+)/i) || [])[1];
    const where = (q.match(/WHERE\s+(.+)/i) || [])[1];
    const filter = {};
    if (where) {
      const match = where.match(/(\w+)\s*=\s*['"]?([^'"]+)['"]?/);
      if (match) filter[match[1]] = match[2];
    }
    return await getModel(m).find(filter).limit(50).lean();
  } else if (verb === "INSERT") {
    // INSERT INTO SecretKey (instanceId,service,regionId,secretKey) VALUES ("x","y","z","k")
    const m = (q.match(/INTO\s+(\w+)/i) || [])[1];
    const fieldsRaw = (q.match(/\(([^)]+)\)/) || [])[1];
    const valuesRaw = (q.match(/VALUES\s*\(([^)]+)\)/i) || [])[1];
    const fields = fieldsRaw.split(",").map(s => s.trim());
    const values = valuesRaw.split(",").map(s => s.trim().replace(/^["']|["']$/g, ""));
    const doc = Object.fromEntries(fields.map((f,i)=>[f, values[i]]));
    return await getModel(m).create(doc);
  } else if (verb === "DELETE") {
    const m = (q.match(/FROM\s+(\w+)/i) || [])[1];
    const where = (q.match(/WHERE\s+(.+)/i) || [])[1];
    const filter = {};
    if (where) {
      const match = where.match(/(\w+)\s*=\s*['"]?([^'"]+)['"]?/);
      if (match) filter[match[1]] = match[2];
    }
    return await getModel(m).deleteMany(filter);
  } else {
    throw new Error("Unsupported SQL-like verb: " + verb);
  }
}
function getModel(name) {
  if (!name) throw new Error("No collection name");
  switch (name) {
    case "SecretKey": return SecretKeyModel;
    case "StatReport": return StatReportModel;
    default: throw new Error("Unknown collection: " + name);
  }
}

// --- SSH Server and REPL ---
const server = new Server({ hostKeys: [hostKey] }, (client) => {
  serverLog("info", "SSH client connected");
  client.on("authentication", (ctx) => {
    if (ctx.method === "password" && ctx.username === username && ctx.password === password) ctx.accept();
    else ctx.reject();
  });

  client.on("ready", () => {
    client.on("session", (accept) => {
      const session = accept();
      session.on("pty", (accept) => accept && accept());
      session.on("shell", (accept) => {
        const stream = accept();
        const promptBase = () => `\x1b[32mducky@NetGoat\x1b[0m:\x1b[34m${cwd}\x1b[0m$ `;
        const r = repl.start({
          input: stream,
          output: stream,
          prompt: promptBase(),
          terminal: true,
          useGlobal: false,
        });

        // built-in commands
        r.context.help = () => {
          return [
            "help - show this",
            "ls, cd, pwd, cat, touch, rm - fake fs",
            "ps, kill - fake process manager",
            "uptime, df, free - fake metrics",
            "addService(service,region,workerId)",
            "removeService(instanceId)",
            "listMachines(service?)",
            "enterDB() - SQL-like mode",
            "BEGIN / COMMIT / ROLLBACK (transaction buffer when in DB mode)",
            "backupAll(tag) / restoreBackup(file) / exportCollection(name) / importCollection(name, json)",
            "aggregateCollection(name, pipelineJsonString)",
            "rotateSecretKey(instanceId) / genToken(instanceId,expires)",
            "simulateLoad(instanceId, cpuPercent, durationMs)",
            "pruneReports(days) / healthCheck()",
            "tailLog(path, lines=20, follow=false)",
            "spawnFakeProcess(name) / killFakeProcess(pid)",
            "scheduleJob(name, delayMs, commandAsString)"
          ].join("\n");
        };

        r.context.ls = (p = ".") => {
          const rp = resolvePath(p);
          const dir = fakeFS[rp] || {};
          return Object.keys(dir).join("  ") || "";
        };
        r.context.pwd = () => cwd;
        r.context.cd = (p = "/") => { cwd = resolvePath(p); r.setPrompt(promptBase()); return cwd; };
        r.context.touch = (name) => { const rp = resolvePath("."); if (!fakeFS[rp]) fakeFS[rp] = {}; fakeFS[rp][name] = ""; return `created ${name}`; };
        r.context.cat = (file) => { const rp = resolvePath("."); return (fakeFS[rp] && fakeFS[rp][file]) || ""; };
        r.context.rm = (file) => { const rp = resolvePath("."); if (fakeFS[rp]) delete fakeFS[rp][file]; return `deleted ${file}`; };

        // process manager
        r.context.ps = () => {
          return processes.map(p => `${p.pid}\t${p.name}\t${new Date(p.startedAt).toISOString()}`).join("\n");
        };
        r.context.spawnFakeProcess = (name) => {
          const proc = { pid: nextPid++, name, startedAt: Date.now() };
          processes.push(proc);
          return `spawned ${proc.pid}`;
        };
        r.context.killFakeProcess = (pid) => {
          const before = processes.length;
          processes = processes.filter(p => p.pid !== Number(pid));
          return before === processes.length ? `no pid ${pid}` : `killed ${pid}`;
        };
        r.context.uptime = () => `${process.uptime().toFixed(1)}s`;
        r.context.df = () => "Filesystem  Size  Used  Avail Use%\n/dev/fake  100G   5G    95G   5%";
        r.context.free = () => `Mem: ${Math.round(os.totalmem()/1024/1024)}MB total`;

        // service management
        r.context.addService = async (service, region, workerId = "default_worker") => {
          const instanceId = `${service}_${region}_${workerId}`;
          const sk = crypto.randomUUID();
          await SecretKeyModel.create({ instanceId, service, workerId, regionId: region, secretKey: sk });
          return `added ${instanceId}`;
        };
        r.context.removeService = async (instanceId) => {
          const res = await SecretKeyModel.deleteOne({ instanceId });
          return res.deletedCount ? `removed ${instanceId}` : `not found`;
        };
        r.context.listMachines = async (service = null) => {
          const filter = service ? { service } : {};
          const rows = await StatReportModel.find(filter).sort({ receivedAt: -1 }).limit(50).lean();
          if (!rows.length) return "no machines";
          const t = new Table({ head: ["instance","cpu%","mem%","lastSeen"] });
          rows.forEach(rp => {
            t.push([`${rp.service}_${rp.regionId}_${rp.workerId}`, rp.stats?.appCpuUsagePercent?.toFixed?.(1) ?? "-", rp.stats?.systemMemoryUsagePercent?.toFixed?.(1) ?? "-", new Date(rp.receivedAt).toLocaleString()]);
          });
          return t.toString();
        };

        // backup/restore/export/import
        r.context.backupAll = async (tag = null) => {
          const p = await backupAllCollections(tag);
          return `wrote backup to ${p}`;
        };
        r.context.restoreBackup = async (filePath) => {
          await restoreFromBackup(filePath, { drop: true });
          return `restored ${filePath}`;
        };
        r.context.exportCollection = async (name) => {
          return await exportCollection(name);
        };
        r.context.importCollection = async (name, jsonText) => {
          const count = await importCollection(name, jsonText);
          return `imported ${count} docs`;
        };
        r.context.aggregateCollection = async (name, pipelineJson) => {
          return await aggregateCollection(name, pipelineJson);
        };

        // keys & tokens
        r.context.rotateSecretKey = async (instanceId) => {
          const doc = await rotateSecretKey(instanceId);
          return doc ? `rotated ${instanceId}` : `not found`;
        };
        r.context.genToken = (instanceId, expires="1h") => generateInstanceJWT(instanceId, expires);

        // simulate load & prune
        r.context.simulateLoad = async (instanceId, cpu=50, durationMs=5000) => {
          const n = await simulateLoad(instanceId, Number(cpu), Number(durationMs));
          return `created ${n} reports`;
        };
        r.context.pruneReports = async (days=30) => {
          const n = await pruneReports(Number(days));
          return `deleted ${n} reports older than ${days}d`;
        };
        r.context.healthCheck = async () => {
          return await healthCheck();
        };

        // tail log
        r.context.tailLog = async (fpath = LOG_FILE, lines = 20, follow = false) => {
          const abs = path.resolve(fpath);
          if (!fs.existsSync(abs)) return `no such file ${abs}`;
          const content = (await fsPromises.readFile(abs, "utf8")).split("\n").slice(-lines).join("\n");
          if (follow) {
            stream.write(content + "\n");
            // simple follow: watch file & stream new lines
            const watcher = fs.watch(abs, async () => {
              const newContent = (await fsPromises.readFile(abs, "utf8")).split("\n").slice(-lines).join("\n");
              stream.write("\n-- file updated --\n" + newContent + "\n");
            });
            // stop following after 30s automatically to avoid leaks
            setTimeout(() => watcher.close(), 30000);
            return `following ${abs} for 30s`;
          }
          return content;
        };

        // SQL-like DB manipulation mode (enterDB)
        r.context.enterDB = () => {
          const prevEval = r.eval;
          r.setPrompt("\x1b[31mdb@\x1b[0m$ ");
          r.eval = async (cmd, ctx, filename, callback) => {
            cmd = (cmd || "").trim();
            if (!cmd) return callback(null);
            if (cmd.toLowerCase() === "exit") {
              r.eval = prevEval;
              r.setPrompt(promptBase());
              return callback(null, "exited DB mode");
            }
            const upper = cmd.toUpperCase();
            if (upper === "BEGIN") { transactionBuffer = []; return callback(null, "transaction started"); }
            if (upper === "COMMIT") {
              const results = [];
              for (const op of transactionBuffer) results.push(await runSQLLike(op));
              transactionBuffer = [];
              return callback(null, prettyJSON(results));
            }
            if (upper === "ROLLBACK") { transactionBuffer = []; return callback(null, "rolled back"); }
            try {
              // If BEGIN has been called, queue instead of executing
              if (transactionBuffer.length > 0 || upper.startsWith("BEGIN")) {
                if (!upper.startsWith("BEGIN")) transactionBuffer.push(cmd);
                return callback(null, "queued");
              }
              const res = await runSQLLike(cmd);
              return callback(null, prettyJSON(res));
            } catch (e) { return callback(null, `${e.name}: ${e.message}`); }
          };

          // completer for SQL-like terms & collection names
          r.completer = (line) => {
            const sqlWords = ["SELECT","INSERT","DELETE","BEGIN","COMMIT","ROLLBACK","EXIT","FROM","WHERE","INTO","VALUES"];
            const cols = ["SecretKey","StatReport"];
            const candidates = [...sqlWords, ...cols];
            return [candidates.filter(c => c.toLowerCase().startsWith(line.toLowerCase())), line];
          };
          return "entered DB mode (SQL-like). Type exit to return.";
        };

        // scheduleJob (simple one-shot)
        r.context.scheduleJob = (name, delayMs = 5000, command = "echo hi") => {
          setTimeout(() => {
            serverLog("info", `Scheduled job ${name} executed: ${command}`);
            // naive: if command equals simulateLoad invocation syntax, try to parse & run
            if (command.startsWith("simulateLoad(")) {
              try {
                const args = command.match(/\((.*)\)/)[1].split(",").map(s=>s.trim().replace(/^['"]|['"]$/g,""));
                simulateLoad(args[0], Number(args[1]||50), Number(args[2]||5000));
              } catch(e){}
            }
          }, Number(delayMs));
          return `scheduled ${name} in ${delayMs}ms`;
        };

        // tab completion across all commands (fallback)
        const globalCommands = ["help","ls","cd","pwd","cat","touch","rm","ps","spawnFakeProcess","killFakeProcess","uptime","df","free","reboot","addService","removeService","listMachines","enterDB","backupAll","restoreBackup","exportCollection","importCollection","aggregateCollection","rotateSecretKey","genToken","simulateLoad","pruneReports","healthCheck","tailLog","scheduleJob","spawnFakeProcess","killFakeProcess"];
        r.completer = (line) => [globalCommands.filter(c => c.startsWith(line)), line];

        r.on("exit", () => {
          stream.end();
        });

      });
    });
  });
});

app.listen(1933)
server.listen(2222, "0.0.0.0", () => serverLog("success", "Fake SSH shell listening on 2222"));
