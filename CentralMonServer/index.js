import { Elysia, t } from "elysia";
import mongoose from "mongoose";
import jwt from "jsonwebtoken";
import chalk from "chalk";
import crypto from "crypto";
import { html } from '@elysiajs/html'; // Import the html plugin


// --- Config ---
const PORT = process.env.PORT || 3000;
const MONGODB_URI =
  process.env.MONGODB_URI || "mongodb://localhost:27017/stats_db";

const SHARED_JWT_SECRET = process.env.SHARED_JWT_SECRET;
const DYNAMIC_SECRET_KEY_JWT_SECRET = process.env.DYNAMIC_SECRET_KEY_JWT_SECRET;

// Logger
const levels = {
  info: { color: chalk.cyan, emoji: "ℹ️ " },
  warn: { color: chalk.yellow, emoji: "⚠️ " },
  error: { color: chalk.red, emoji: "❌ " },
  debug: { color: chalk.magenta, emoji: "🐛 " },
  success: { color: chalk.green, emoji: "✅" },
};
const serverLog = (level, ...msg) => {
  const { color, emoji } = levels[level] || levels.info;
  const timestamp = chalk.gray(new Date().toISOString());
  console.log(
    `${emoji} ${timestamp} ${color.bold(level.toUpperCase())} › [Server]`,
    ...msg
  );
};

// MongoDB connection
mongoose
  .connect(MONGODB_URI)
  .then(() => serverLog("success", `Connected to MongoDB at ${MONGODB_URI}`))
  .catch((err) => serverLog("error", "MongoDB connection error:", err));

// Schemas
const secretKeySchema = new mongoose.Schema({
  instanceId: { type: String, required: true, unique: true },
  service: { type: String, required: true },
  workerId: { type: String, default: "default_worker" },
  regionId: { type: String, required: true },
  secretKey: { type: String, required: true },
  createdAt: { type: Date, default: Date.now },
  updatedAt: { type: Date, default: Date.now },
});
const SecretKeyModel = mongoose.model("SecretKey", secretKeySchema);

const statReportSchema = new mongoose.Schema({
  dataKey: { type: String, required: true },
  service: { type: String, required: true },
  workerId: { type: String, default: "default_worker" },
  regionId: { type: String, required: true },
  stats: {
    timestamp: { type: Date, required: true },
    hostname: String,
    platform: String,
    arch: String,
    systemUptimeSeconds: Number,
    cpuCount: Number,
    cpuModel: String,
    systemLoadAverage: [Number],
    totalSystemMemoryBytes: Number,
    freeSystemMemoryBytes: Number,
    usedSystemMemoryBytes: Number,
    systemMemoryUsagePercent: Number,
    processId: Number,
    processUptimeSeconds: Number,
    appCpuUsagePercent: Number,
    appMemoryRssBytes: Number,
  },
  receivedAt: { type: Date, default: Date.now },
});
statReportSchema.index({ dataKey: 1, receivedAt: -1 });
statReportSchema.index({ service: 1, regionId: 1, receivedAt: -1 });
statReportSchema.index({ service: 1, workerId: 1, receivedAt: -1 });
const StatReportModel = mongoose.model("StatReport", statReportSchema);

// App

const app = new Elysia()
  .use(html()) // Use the html plugin
  .post(
    "/auth",
    async ({ body, headers, set }) => {
      const { service, workerId, regionId } = body;
      const authHeader = headers.authorization;

      if (!authHeader || !authHeader.startsWith("Bearer ")) {
        set.status = 401;
        serverLog(
          "warn",
          "Authentication failed: Missing or invalid Authorization header."
        );
        return {
          message: "Unauthorized: Missing or invalid Authorization header",
        };
      }

      const sharedJwtToken = authHeader.split(" ")[1];

      try {
        const decoded = jwt.verify(sharedJwtToken, SHARED_JWT_SECRET);
        serverLog("debug", "SHARED_JWT verified:", decoded);

        if (!service || !regionId) {
          set.status = 400;
          serverLog(
            "warn",
            "Authentication failed: Missing service or regionId in request body."
          );
          return { message: "Bad Request: service and regionId are required." };
        }

        const instanceId =
          workerId && workerId !== "default_worker"
            ? `${service}_${regionId}_${workerId}`
            : `${service}_${regionId}`;

        let secretKeyRecord = await SecretKeyModel.findOne({ instanceId });

        if (!secretKeyRecord) {
          const newSecretKey = crypto.randomUUID();
          secretKeyRecord = new SecretKeyModel({
            instanceId,
            service,
            workerId: workerId || "default_worker",
            regionId,
            secretKey: newSecretKey,
          });
          await secretKeyRecord.save();
          serverLog(
            "success",
            `Generated and stored new SecretKey for instance: ${instanceId}`
          );
        } else {
          serverLog(
            "info",
            `Using existing SecretKey for instance: ${instanceId}`
          );
        }

        const token = jwt.sign(
          { secretKey: secretKeyRecord.secretKey, instanceId: instanceId },
          DYNAMIC_SECRET_KEY_JWT_SECRET,
          { expiresIn: "1h" }
        );

        set.status = 200;
        return { token };
      } catch (error) {
        set.status = 401;
        serverLog("error", "Authentication failed:", error.message);
        return { message: "Unauthorized: Invalid SHARED_JWT or server error." };
      }
    },
    {
      body: t.Object({
        service: t.String(),
        workerId: t.Optional(t.String()),
        regionId: t.String(),
      }),
    }
  )

  // Report stats endpoint
  .post(
    "/report-stats",
    async ({ body, headers, set }) => {
      const secretKeyHeader = headers["x-secret-key"];

      if (!secretKeyHeader) {
        set.status = 401;
        serverLog("warn", "Stats report failed: Missing X-Secret-Key header.");
        return { message: "Unauthorized: Missing X-Secret-Key header" };
      }

      const { dataKey, service, workerId, regionId, stats } = body;

      if (!dataKey || !service || !regionId || !stats) {
        set.status = 400;
        serverLog(
          "warn",
          "Stats report failed: Missing required fields in body.",
          body
        );
        return {
          message:
            "Bad Request: dataKey, service, regionId, and stats are required.",
        };
      }

      try {
        const instanceId =
          workerId && workerId !== "default_worker"
            ? `${service}_${regionId}_${workerId}`
            : `${service}_${regionId}`;

        const secretKeyRecord = await SecretKeyModel.findOne({ instanceId });

        if (!secretKeyRecord || secretKeyRecord.secretKey !== secretKeyHeader) {
          set.status = 403;
          serverLog(
            "warn",
            `Stats report failed: Invalid SecretKey for instance ${instanceId}.`
          );
          return { message: "Forbidden: Invalid SecretKey" };
        }

        const newStatReport = new StatReportModel({
          dataKey,
          service,
          workerId: workerId || "default_worker",
          regionId,
          stats,
        });
        await newStatReport.save();

        serverLog("info", `Received and stored stats for ${dataKey}`);
        set.status = 200;
        return { message: "Stats received and stored successfully!" };
      } catch (error) {
        set.status = 500;
        serverLog("error", "Error processing stats report:", error.message);
        return { message: "Internal Server Error" };
      }
    },
    {
      body: t.Object({
        dataKey: t.String(),
        service: t.String(),
        workerId: t.Optional(t.String()),
        regionId: t.String(),
        stats: t.Object({
          timestamp: t.String(),
          hostname: t.String(),
          platform: t.String(),
          arch: t.String(),
          systemUptimeSeconds: t.Number(),
          cpuCount: t.Number(),
          cpuModel: t.String(),
          systemLoadAverage: t.Array(t.Number()),
          totalSystemMemoryBytes: t.Number(),
          freeSystemMemoryBytes: t.Number(),
          usedSystemMemoryBytes: t.Number(),
          systemMemoryUsagePercent: t.Number(),
          processId: t.Number(),
          processUptimeSeconds: t.Number(),
          appCpuUsagePercent: t.Number(),
          appMemoryRssBytes: t.Number(),
        }),
      }),
    }
  )

  // Stats fetch API for UI
  .get("/api/stats", async ({ query, set }) => {
    const { service, region, limit = 20 } = query;

    try {
      const filter = {};
      if (service) filter.service = service;
      if (region) filter.regionId = region;

      const reports = await StatReportModel.find(filter)
        .sort({ receivedAt: -1 })
        .limit(Number(limit))
        .lean();

      set.status = 200;
      return { reports };
    } catch (error) {
      serverLog("error", "Failed to fetch stats:", error.message);
      set.status = 500;
      return { message: "Internal Server Error" };
    }
  })

  // Serve simple monitoring UI
  .get(
    "/",
    () =>
      `
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>NetGoat Stats Monitor</title>
<style>
  body { font-family: system-ui, sans-serif; background: #111; color: #eee; padding: 20px; }
  table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
  th, td { padding: 8px; border: 1px solid #444; text-align: left; }
  th { background: #222; }
  tr:nth-child(even) { background: #222; }
  input, button { margin: 5px; padding: 5px; }
</style>
</head>
<body>
<h1>NetGoat Stats Monitor</h1>

<label>Service:
  <input id="serviceInput" type="text" placeholder="Filter service" />
</label>
<label>Region:
  <input id="regionInput" type="text" placeholder="Filter region" />
</label>
<button onclick="loadStats()">Refresh</button>

<table>
  <thead>
    <tr>
      <th>Data Key</th>
      <th>CPU %</th>
      <th>Memory %</th>
      <th>Uptime (s)</th>
      <th>Received At</th>
    </tr>
  </thead>
  <tbody id="statsTableBody"></tbody>
</table>

<script>
  async function loadStats() {
    const service = document.getElementById('serviceInput').value;
    const region = document.getElementById('regionInput').value;
    let url = '/api/stats?limit=30';
    if (service) url += '&service=' + encodeURIComponent(service);
    if (region) url += '&region=' + encodeURIComponent(region);
    
    const res = await fetch(url);
    const data = await res.json();

    const tbody = document.getElementById('statsTableBody');
    tbody.innerHTML = '';
    data.reports.forEach(r => {
      const tr = document.createElement('tr');
      tr.innerHTML = \`
        <td>\${r.dataKey}</td>
        <td>\${r.stats.appCpuUsagePercent.toFixed(2)}</td>
        <td>\${r.stats.systemMemoryUsagePercent.toFixed(2)}</td>
        <td>\${r.stats.systemUptimeSeconds.toFixed(0)}</td>
        <td>\${new Date(r.receivedAt).toLocaleString()}</td>
      \`;
      tbody.appendChild(tr);
    });
  }

  loadStats();
  setInterval(loadStats, 30000);
</script>
</body>
</html>
`
  )

  .listen(PORT, () => {
    serverLog("success", `ElysiaJS server running on http://localhost:${PORT}`);
  });
