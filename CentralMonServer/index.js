import { Elysia, t } from "elysia";
import mongoose from "mongoose";
import jwt from "jsonwebtoken";
import chalk from "chalk";
import crypto from "crypto";
import { html } from "@elysiajs/html";
import { staticPlugin } from "@elysiajs/static";
import fs from "fs/promises";
import os from "os";
import util from "util";
import path from "path";
import bcrypt from "bcrypt";

const PORT = process.env.PORT || 1933;
const MONGODB_URI =
  process.env.MONGODB_URI || "mongodb://localhost:27017/stats_db";
const SHARED_JWT_SECRET = process.env.SHARED_JWT_SECRET || "shared_secret";
const DYNAMIC_SECRET_KEY_JWT_SECRET =
  process.env.DYNAMIC_SECRET_KEY_JWT_SECRET || "dynamic_secret";
const ADMIN_JWT_SECRET = process.env.ADMIN_JWT_SECRET || "admin_secret";
const LOG_FILE = process.env.LOG_FILE || "./server.log";
const BACKUP_DIR = process.env.BACKUP_DIR || "./backups";
const ALLOWED_REGIONS = ["MM", "sg", "id"];
const ALLOWED_CATEGORIES = ["main", "logdb", "sidecar"];

const serverLog = (lvl, ...m) => {
  const ts = new Date().toISOString();
  const out = `${ts} ${lvl.toUpperCase()} › ${m.join(" ")}`;
  const color =
    {
      info: chalk.cyan,
      warn: chalk.yellow,
      error: chalk.red,
      success: chalk.green,
    }[lvl] || chalk.white;
  console.log(color(out));
  fs.appendFile(LOG_FILE, out + "\n").catch(() => {});
};

let DB_READY = false;
await fs.mkdir(BACKUP_DIR, { recursive: true }).catch(() => {});
await mongoose
  .connect(MONGODB_URI, { serverSelectionTimeoutMS: 5000 })
  .then(() => {
    DB_READY = true;
    serverLog("success", `Mongo connected ${MONGODB_URI}`);
  })
  .catch((e) => serverLog("error", "mongo connect:", e.message));

const secretKeySchema = new mongoose.Schema({
  instanceId: { type: String },
  service: String,
  workerId: { type: String, default: "default_worker" },
  regionId: String,
  category: String,
  secretKey: String,
  createdAt: { type: Date, default: Date.now },
  updatedAt: { type: Date, default: Date.now },
});
secretKeySchema.index({ instanceId: 1 }, { unique: true });
const SecretKeyModel = mongoose.model("SecretKey", secretKeySchema);

const statReportSchema = new mongoose.Schema({
  dataKey: String,
  service: String,
  workerId: { type: String, default: "default_worker" },
  regionId: String,
  category: String,
  stats: Object,
  receivedAt: { type: Date, default: Date.now },
});
statReportSchema.index({ dataKey: 1, receivedAt: -1 });
statReportSchema.index({ service: 1, regionId: 1, receivedAt: -1 });
const StatReportModel = mongoose.model("StatReport", statReportSchema);

const historySchema = new mongoose.Schema({
  status: String,
  responseTime: Number,
  timestamp: { type: Date, default: Date.now },
  category: String,
  regionId: String,
  service: String,
  details: Object,
});
historySchema.index(
  { timestamp: 1 },
  { expireAfterSeconds: 60 * 60 * 24 * 90 }
);
const HistoryModel = mongoose.model("History", historySchema);

const adminSchema = new mongoose.Schema({
  username: { type: String, unique: true },
  passwordHash: String,
  createdAt: { type: Date, default: Date.now },
});
const AdminModel = mongoose.model("Admin", adminSchema);

const pretty = (v) => util.inspect(v, { depth: 5 });

async function addHistory(payload) {
  try {
    await HistoryModel.create(payload);
  } catch (e) {
    serverLog("error", "history write failed", e.message);
  }
}

async function ensureAdminFromEnv() {
  if (!DB_READY) return;
  const count = await AdminModel.countDocuments();
  if (count === 0) {
    const uname = process.env.ADMIN_USERNAME;
    const pass = process.env.ADMIN_PASSWORD;
    if (!uname || !pass) {
      serverLog(
        "warn",
        "no admin in DB and ADMIN_USERNAME/ADMIN_PASSWORD not provided"
      );
      return;
    }
    const hash = await bcrypt.hash(pass, 10);
    await AdminModel.create({ username: uname, passwordHash: hash });
    serverLog("success", "initial admin created");
  }
}
ensureAdminFromEnv().catch(() => {});

async function backupAllCollections(tag = null) {
  const time = new Date().toISOString().replace(/[.:]/g, "-");
  const name = `${tag || "dump"}-${time}.json`;
  const p = path.join(BACKUP_DIR, name);
  const out = {
    SecretKey: await SecretKeyModel.find({}).lean(),
    StatReport: await StatReportModel.find({}).lean(),
    History: await HistoryModel.find({}).lean(),
  };
  await fs.writeFile(p, JSON.stringify(out, null, 2));
  await addHistory({
    status: "backup_created",
    responseTime: 0,
    category: "logdb",
    regionId: "sys",
    service: "backup",
    details: { path: p },
  });
  return p;
}

async function rotateSecretKey(instanceId) {
  const newKey = crypto.randomUUID();
  const doc = await SecretKeyModel.findOneAndUpdate(
    { instanceId },
    { secretKey: newKey, updatedAt: new Date() },
    { new: true }
  );
  if (doc)
    await addHistory({
      status: "rotate_secret",
      responseTime: 0,
      category: doc.category || "main",
      regionId: doc.regionId,
      service: doc.service,
      details: { instanceId },
    });
  return doc;
}

function genInstanceToken(instanceId, secretKey, expires = "1h") {
  return jwt.sign({ instanceId, secretKey }, DYNAMIC_SECRET_KEY_JWT_SECRET, {
    expiresIn: expires,
  });
}

async function runSQLLike(q) {
  const verb = q.trim().split(/\s+/)[0]?.toUpperCase();
  if (verb === "SELECT") {
    const m = (q.match(/FROM\s+(\w+)/i) || [])[1];
    const where = (q.match(/WHERE\s+(.+)/i) || [])[1];
    const filter = {};
    if (where) {
      const match = where.match(/(\w+)\s*=\s*['"]?([^'"]+)['"]?/);
      if (match) filter[match[1]] = match[2];
    }
    return await (m === "StatReport"
      ? StatReportModel
      : m === "SecretKey"
      ? SecretKeyModel
      : HistoryModel
    )
      .find(filter)
      .limit(200)
      .lean();
  }
  throw new Error("unsupported");
}

const app = new Elysia().use(html());


app.get("/api/health", async () => {
  await addHistory({
    status: "health_ping",
    responseTime: 0,
    category: "main",
    regionId: "sys",
    service: "server",
  });
  return { status: "ok", uptime: process.uptime() };
});

app.post(
  "/admin/login",
  async ({ body, set }) => {
    const { username, password } = body;
    if (!username || !password) {
      set.status = 400;
      return { error: "username+password required" };
    }
    if (!DB_READY) {
      set.status = 503;
      return { error: "db not ready" };
    }
    const admin = await AdminModel.findOne({ username });
    if (!admin) {
      set.status = 401;
      return { error: "invalid" };
    }
    const ok = await bcrypt.compare(password, admin.passwordHash);
    if (!ok) {
      set.status = 401;
      return { error: "invalid" };
    }
    const token = jwt.sign(
      { sub: admin._id, u: admin.username },
      ADMIN_JWT_SECRET,
      { expiresIn: "8h" }
    );
    await addHistory({
      status: "admin_login",
      responseTime: 0,
      category: "main",
      regionId: "sys",
      service: "admin",
      details: { user: admin.username },
    });
    return { token };
  },
  { body: t.Object({ username: t.String(), password: t.String() }) }
);

function requireAdmin(req) {
  const h = req.headers.authorization;
  if (!h || !h.startsWith("Bearer ")) throw new Error("unauth");
  try {
    return jwt.verify(h.split(" ")[1], ADMIN_JWT_SECRET);
  } catch {
    throw new Error("unauth");
  }
}

app.post(
  "/auth",
  async ({ body, headers, set }) => {
    if (!DB_READY) {
      set.status = 503;
      return { message: "db not ready" };
    }
    try {
      const authHeader = headers.authorization;
      if (!authHeader || !authHeader.startsWith("Bearer ")) {
        set.status = 401;
        return { message: "Unauthorized" };
      }
      jwt.verify(authHeader.split(" ")[1], SHARED_JWT_SECRET);
      const { service, workerId, regionId, category } = body;
      if (!service || !regionId) {
        set.status = 400;
        return { message: "Missing service/regionId" };
      }
      if (!ALLOWED_REGIONS.includes(regionId)) {
        set.status = 400;
        return { message: "Invalid region" };
      }
      const cat = category || "main";
      if (!ALLOWED_CATEGORIES.includes(cat)) {
        set.status = 400;
        return { message: "Invalid category" };
      }
      const instanceId =
        workerId && workerId !== "default_worker"
          ? `${service}_${regionId}_${workerId}`
          : `${service}_${regionId}`;

      // **PERSISTENT KEY** – only create if not exists
      let sk = await SecretKeyModel.findOneAndUpdate(
        { instanceId },
        {
          $setOnInsert: {
            instanceId,
            service,
            workerId: workerId || "default_worker",
            regionId,
            category: cat,
            secretKey: crypto.randomUUID(),
            createdAt: new Date(),
            updatedAt: new Date(),
          },
        },
        { new: true, upsert: true }
      );

      const token = genInstanceToken(instanceId, sk.secretKey);
      await addHistory({
        status: "auth_success",
        responseTime: 0,
        category: cat,
        regionId,
        service,
      });
      return { token, secretKey: sk.secretKey };
    } catch (e) {
      set.status = 401;
      await addHistory({
        status: "auth_exception",
        responseTime: 0,
        details: { message: e.message },
      });
      return { message: "Unauthorized" };
    }
  },
  {
    body: t.Object({
      service: t.String(),
      workerId: t.Optional(t.String()),
      regionId: t.String(),
      category: t.Optional(t.String()),
    }),
  }
);

app.post(
  "/report-stats",
  async ({ body, headers, set }) => {
    if (!DB_READY) {
      set.status = 503;
      return { message: "db not ready" };
    }
    try {
      const secretKeyHeader = headers["x-secret-key"];
      if (!secretKeyHeader) {
        set.status = 401;
        return { message: "Missing X-Secret-Key" };
      }
      const { dataKey, service, workerId, regionId, stats, category } = body;
      if (!dataKey || !service || !regionId || !stats) {
        set.status = 400;
        return { message: "Missing fields" };
      }
      if (!ALLOWED_REGIONS.includes(regionId)) {
        set.status = 400;
        return { message: "Invalid region" };
      }
      const cat = category || "main";
      if (!ALLOWED_CATEGORIES.includes(cat)) {
        set.status = 400;
        return { message: "Invalid category" };
      }
      const instanceId =
        workerId && workerId !== "default_worker"
          ? `${service}_${regionId}_${workerId}`
          : `${service}_${regionId}`;
      const sk = await SecretKeyModel.findOne({ instanceId });
      if (!sk || sk.secretKey !== secretKeyHeader) {
        set.status = 403;
        return { message: "Forbidden" };
      }
      await StatReportModel.create({
        dataKey,
        service,
        workerId: workerId || "default_worker",
        regionId,
        category: cat,
        stats,
      });
      await addHistory({
        status: "report_stored",
        responseTime: 0,
        category: cat,
        regionId,
        service,
      });
      return { message: "Stored" };
    } catch (e) {
      set.status = 500;
      await addHistory({
        status: "report_error",
        responseTime: 0,
        details: { message: e.message },
      });
      return { message: "Error" };
    }
  },
  {
    body: t.Object({
      dataKey: t.String(),
      service: t.String(),
      workerId: t.Optional(t.String()),
      regionId: t.String(),
      stats: t.Any(),
      category: t.Optional(t.String()),
    }),
  }
);

app.get("/api/stats", async ({ query }) => {
  const { service, region, category, limit = 50 } = query;
  const filter = {};
  if (service) filter.service = service;
  if (region) filter.regionId = region;
  if (category) filter.category = category;
  const reports = await StatReportModel.find(filter)
    .sort({ receivedAt: -1 })
    .limit(Number(limit))
    .lean();
  return { reports };
});

app.get("/api/history", async ({ query }) => {
  const { category, region, service, limit = 200 } = query;
  const filter = {};
  if (category) filter.category = category;
  if (region) filter.regionId = region;
  if (service) filter.service = service;
  const history = await HistoryModel.find(filter)
    .sort({ timestamp: -1 })
    .limit(Number(limit))
    .lean();
  return { history };
});

app.listen(PORT, () => {
  serverLog("success", `Elysia listening ${PORT}`);
  addHistory({
    status: "server_started",
    responseTime: 0,
    category: "main",
    regionId: "sys",
    service: "server",
  });
});
