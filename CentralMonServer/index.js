import { Elysia, t } from "elysia";
import mongoose from "mongoose";
import fetch from "node-fetch";
import chalk from "chalk";
import fs from "fs/promises";
import crypto from "crypto";
import { cors } from "@elysiajs/cors";

const PORT = process.env.PORT || 1933;
const MONGODB_URI =
  process.env.MONGODB_URI || "mongodb://localhost:27017/stats_db";
const LOG_FILE = process.env.LOG_FILE || "./server.log";
const ALLOWED_REGIONS = ["MM", "sg", "id"];
const ALLOWED_CATEGORIES = ["main", "logdb", "sidecar"];

let DB_READY = false;

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

await mongoose
  .connect(MONGODB_URI, { serverSelectionTimeoutMS: 5000 })
  .then(() => {
    DB_READY = true;
    serverLog("success", `Mongo connected ${MONGODB_URI}`);
  })
  .catch((e) => serverLog("error", "mongo connect:", e.message));

// History Model
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

// SecretKey Model with endpoint
const secretKeySchema = new mongoose.Schema({
  instanceId: { type: String, unique: true },
  service: String,
  workerId: { type: String, default: "default_worker" },
  regionId: String,
  category: String,
  secretKey: String,
  endpoint: String,
  createdAt: { type: Date, default: Date.now },
  updatedAt: { type: Date, default: Date.now },
});
const SecretKeyModel = mongoose.model("SecretKey", secretKeySchema);

// StatReport Model
const statReportSchema = new mongoose.Schema({
  dataKey: String,
  service: String,
  workerId: { type: String, default: "default_worker" },
  regionId: String,
  category: String,
  stats: Object,
  endpoint: String,
  receivedAt: { type: Date, default: Date.now },
});
const StatReportModel = mongoose.model("StatReport", statReportSchema);

// Utility
async function addHistory(payload) {
  try {
    await HistoryModel.create(payload);
  } catch (e) {
    serverLog("error", "history write failed", e.message);
  }
}

// Ping a single service
async function pingService(svc) {
  if (!svc.endpoint) return;
  const start = Date.now();
  try {
    const res = await fetch(svc.endpoint, { timeout: 3000 });
    const latency = Date.now() - start;
    await addHistory({
      status: "health_ping",
      responseTime: latency,
      category: svc.category || "main",
      regionId: svc.regionId,
      service: svc.service,
      details: { endpoint: svc.endpoint, ok: res.ok },
    });
  } catch (e) {
    await addHistory({
      status: "health_ping_fail",
      responseTime: 0,
      category: svc.category || "main",
      regionId: svc.regionId,
      service: svc.service,
      details: { endpoint: svc.endpoint, error: e.message },
    });
  }
}

// Periodic ping all registered services
setInterval(async () => {
  if (!DB_READY) return;
  const services = await SecretKeyModel.find({
    endpoint: { $exists: true },
  }).lean();
  await Promise.all(services.map(pingService));
}, 10000); // every 10s

// Elysia server
const app = new Elysia();
app.use(
  cors({
    origin: "*", // allow all origins
    methods: ["GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"], // allowed methods
    headers: ["Content-Type", "Authorization"], // allowed headers
  })
);
// Monitoring server health
app.get("/api/health", async () => {
  await addHistory({
    status: "server_health_ping",
    responseTime: 0,
    category: "main",
    regionId: "sys",
    service: "monitor",
  });
  return { status: "ok", uptime: process.uptime() };
});

app.get("/api/health-pings", async (req, res) => {
  const data = await db
    .collection("history")
    .find({ status: "health_ping" })
    .sort({ timestamp: -1 })
    .limit(100)
    .toArray();
  res.json(data);
});

// Register microservice
app.post(
  "/auth",
  async ({ body, set }) => {
    const { service, workerId, regionId, category, endpoint } = body;
    if (!service || !regionId || !endpoint) {
      set.status = 400;
      return { message: "Missing fields or endpoint" };
    }
    const cat = category || "main";
    const instanceId =
      workerId && workerId !== "default_worker"
        ? `${service}_${regionId}_${workerId}`
        : `${service}_${regionId}`;
    const sk = await SecretKeyModel.findOneAndUpdate(
      { instanceId },
      {
        $setOnInsert: {
          instanceId,
          service,
          workerId: workerId || "default_worker",
          regionId,
          category: cat,
          secretKey: crypto.randomUUID(),
          endpoint,
          createdAt: new Date(),
          updatedAt: new Date(),
        },
      },
      { new: true, upsert: true }
    );
    await addHistory({
      status: "auth_success",
      responseTime: 0,
      category: cat,
      regionId,
      service,
    });
    return { token: sk.secretKey, instanceId };
  },
  {
    body: t.Object({
      service: t.String(),
      workerId: t.Optional(t.String()),
      regionId: t.String(),
      category: t.Optional(t.String()),
      endpoint: t.String(),
    }),
  }
);

// Report stats and ping immediately
app.post(
  "/report-stats",
  async ({ body, headers, set }) => {
    const { service, workerId, regionId, stats, category, endpoint } = body;
    if (!service || !regionId || !stats) {
      set.status = 400;
      return { message: "Missing fields" };
    }
    const cat = category || "main";
    const instanceId =
      workerId && workerId !== "default_worker"
        ? `${service}_${regionId}_${workerId}`
        : `${service}_${regionId}`;
    const sk = await SecretKeyModel.findOne({ instanceId });
    if (!sk) {
      set.status = 403;
      return { message: "Forbidden" };
    }

    await StatReportModel.create({
      dataKey: crypto.randomUUID(),
      service,
      workerId: workerId || "default_worker",
      regionId,
      category: cat,
      stats,
      endpoint: endpoint || sk.endpoint,
    });

    // Ping the service immediately
    await pingService({ ...sk, endpoint: endpoint || sk.endpoint });

    await addHistory({
      status: "report_stored",
      responseTime: 0,
      category: cat,
      regionId,
      service,
    });
    return { message: "Stored" };
  },
  {
    body: t.Object({
      service: t.String(),
      workerId: t.Optional(t.String()),
      regionId: t.String(),
      stats: t.Any(),
      category: t.Optional(t.String()),
      endpoint: t.Optional(t.String()),
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
