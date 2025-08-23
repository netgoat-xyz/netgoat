import { Elysia, t } from "elysia";
import mongoose from "mongoose";
import jwt from "jsonwebtoken";
import chalk from "chalk";
import crypto from "crypto";
import { html } from '@elysiajs/html'; // Import the html plugin


// --- Config ---
const PORT = process.env.PORT || 1933;
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
  <script src="https://cdn.tailwindcss.com"></script>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;600&display=swap" rel="stylesheet">
  <style>
    body { font-family: 'Inter', system-ui, sans-serif; }
    .fade-in { animation: fadeIn 0.7s cubic-bezier(.4,0,.2,1); }
    @keyframes fadeIn { from { opacity: 0; transform: translateY(16px);} to { opacity: 1; transform: none; } }
  </style>
</head>
<body class="bg-gradient-to-br from-zinc-900 to-zinc-800 min-h-screen text-zinc-100 px-4 py-8">
  <div class="max-w-5xl mx-auto fade-in">
    <h1 class="text-3xl font-bold mb-2 tracking-tight">NetGoat <span class="text-primary-500">Stats Monitor</span></h1>
    <p class="text-zinc-400 mb-6">Monitor and filter your service stats in real time.</p>
    <div class="flex flex-col sm:flex-row gap-3 mb-6">
      <input id="serviceInput" type="text" placeholder="Filter service" class="rounded-lg border border-zinc-700 bg-zinc-900 px-4 py-2 text-zinc-100 focus:outline-none focus:ring-2 focus:ring-primary-500 transition" />
      <input id="regionInput" type="text" placeholder="Filter region" class="rounded-lg border border-zinc-700 bg-zinc-900 px-4 py-2 text-zinc-100 focus:outline-none focus:ring-2 focus:ring-primary-500 transition" />
      <button onclick="loadStats()" class="rounded-lg bg-primary-600 hover:bg-primary-500 transition text-white px-5 py-2 font-semibold shadow-sm">Refresh</button>
    </div>
    <div class="overflow-x-auto rounded-xl shadow-lg bg-zinc-900/80 backdrop-blur border border-zinc-800">
      <table class="min-w-full divide-y divide-zinc-800">
        <thead>
          <tr>
            <th class="px-4 py-3 text-left text-xs font-semibold text-zinc-400 uppercase tracking-wider">Service</th>
            <th class="px-4 py-3 text-left text-xs font-semibold text-zinc-400 uppercase tracking-wider">Region</th>
            <th class="px-4 py-3 text-left text-xs font-semibold text-zinc-400 uppercase tracking-wider">Worker ID</th>
            <th class="px-4 py-3 text-left text-xs font-semibold text-zinc-400 uppercase tracking-wider">CPU %</th>
            <th class="px-4 py-3 text-left text-xs font-semibold text-zinc-400 uppercase tracking-wider">Memory %</th>
            <th class="px-4 py-3 text-left text-xs font-semibold text-zinc-400 uppercase tracking-wider">Uptime (s)</th>
            <th class="px-4 py-3 text-left text-xs font-semibold text-zinc-400 uppercase tracking-wider">Received At</th>
          </tr>
        </thead>
        <tbody id="statsTableBody" class="divide-y divide-zinc-800"></tbody>
      </table>
    </div>
    <div id="emptyState" class="hidden text-center text-zinc-500 py-8">No stats found for the current filter.</div>
  </div>
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
      const emptyState = document.getElementById('emptyState');
      tbody.innerHTML = '';
      if (!data.reports || data.reports.length === 0) {
        emptyState.classList.remove('hidden');
        return;
      } else {
        emptyState.classList.add('hidden');
      }
      data.reports.forEach((r, i) => {
        // Split dataKey: Service_Region_WorkerID
        let service = '-', region = '-', worker = '-';
        if (typeof r.dataKey === 'string') {
          const parts = r.dataKey.split('_');
          if (parts.length === 3) {
            [service, region, worker] = parts;
          } else if (parts.length === 2) {
            [service, region] = parts;
          } else {
            service = r.dataKey;
          }
        }
        const tr = document.createElement('tr');
        tr.className = 'hover:bg-zinc-800/60 transition';
        tr.innerHTML = \`
          <td class="px-4 py-3 whitespace-nowrap font-mono text-sm">\${service}</td>
          <td class="px-4 py-3 whitespace-nowrap font-mono text-sm">\${region}</td>
          <td class="px-4 py-3 whitespace-nowrap font-mono text-sm">\${worker}</td>
          <td class="px-4 py-3">\${r.stats.appCpuUsagePercent !== undefined ? r.stats.appCpuUsagePercent.toFixed(2) : '-'}</td>
          <td class="px-4 py-3">\${r.stats.systemMemoryUsagePercent !== undefined ? r.stats.systemMemoryUsagePercent.toFixed(2) : '-'}</td>
          <td class="px-4 py-3">\${r.stats.systemUptimeSeconds !== undefined ? r.stats.systemUptimeSeconds.toFixed(0) : '-'}</td>
          <td class="px-4 py-3 text-zinc-400">\${new Date(r.receivedAt).toLocaleString()}</td>
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
