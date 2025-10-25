import os from "os";
import process from "process";
import logger from "./logger"

let reportingInterval = null;
let currentSecretKey = null;
let lastCpuUsage = process.cpuUsage();
let lastHr = process.hrtime.bigint();

function log(level, ...args) {
  if (level === "debug" && process.env.DEBUG !== "true") return; // only log debug if DEBUG=true
  logger.stats(...args);
}

async function sampleAppCpu() {
  await new Promise((r) => setTimeout(r, 30));
  const nowCpu = process.cpuUsage();
  const nowHr = process.hrtime.bigint();

  const cpuDiff =
    nowCpu.user - lastCpuUsage.user + (nowCpu.system - lastCpuUsage.system);
  const hrDiff = Number(nowHr - lastHr) / 1000; // µs

  lastCpuUsage = nowCpu;
  lastHr = nowHr;
  return hrDiff === 0 ? 0 : +((cpuDiff / hrDiff) * 100).toFixed(2);
}

async function collectStats() {
  const total = os.totalmem();
  const free = os.freemem();
  return {
    ts: new Date().toISOString(),
    host: os.hostname(),
    arch: os.arch(),
    platform: os.platform(),
    sysUptime: os.uptime(),
    cpuCount: os.cpus().length,
    cpuModel: os.cpus()[0].model,
    load: os.loadavg(),
    mem: {
      total,
      used: total - free,
      usedPct: +(((total - free) / total) * 100).toFixed(2),
    },
    proc: {
      pid: process.pid,
      uptime: process.uptime(),
      rss: process.memoryUsage().rss,
      cpuPct: await sampleAppCpu(),
    },
  };
}

function decodeJwt(token) {
  try {
    const payload = token.split(".")[1];
    return JSON.parse(Buffer.from(payload, "base64").toString());
  } catch (err) {
    log("error", "JWT decode failed:", err.message);
    return null;
  }
}

function getServiceEndpoint() {
  if (!process.env.NODE_ENV) process.env.NODE_ENV = "development";
  if (process.env.NODE_ENV === "development") {
    return `http://localhost:${process.env.PORT || 3001}/api/health`;
  } else {
    // use HOSTNAME env if set, otherwise find first non-internal IPv4
    let host = process.env.HOSTNAME;
    if (!host) {
      const nets = os.networkInterfaces();
      outer: for (const name of Object.keys(nets)) {
        for (const net of nets[name]) {
          if (net.family === "IPv4" && !net.internal) {
            host = net.address;
            break outer;
          }
        }
      }
    }
    const port = process.env.PORT || 3001;
    return `http://${host}:${port}/api/health`;
  }
}

async function auth(
  serverUrl,
  sharedJwt,
  service,
  category,
  regionId,
  workerId
) {
  const endpoint = `${serverUrl}/auth`;
  log("info", `Authenticating with ${endpoint}`);
  try {
    const r = await fetch(endpoint, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${sharedJwt}`,
      },
      body: JSON.stringify({
        service,
        category,
        regionId,
        workerId,
        endpoint: getServiceEndpoint(),
      }),
    });
    if (!r.ok) {
      log("error", `Auth failed: ${r.status} ${r.statusText}`);
      log("debug", "Auth response:", await r.text());
      return null;
    }

    // server returns { token: "<secretKey>" } directly
    const { token } = await r.json();
    if (!token) {
      log("error", "Auth response missing token/secretKey");
      return null;
    }

    log("success", "Obtained SecretKey:", token);
    return token; // use this directly
  } catch (err) {
    log("error", "Auth error:", err.message);
    return null;
  }
}

export async function startReporting({
  serverUrl,
  sharedJwt,
  intervalMinutes = 1,
  service,
  category = "logdb",
  regionId = "mm",
  workerId = "default_worker",
  maxRetries = 5,
  retryDelayMs = 10_000,
}) {
  if (reportingInterval) {
    log("warn", "Reporter already running, stopping old interval.");
    stopReporting();
  }

  currentSecretKey = await auth(
    serverUrl,
    sharedJwt,
    service,
    category,
    regionId,
    workerId
  );
  if (!currentSecretKey) {
    log("error", "Initial authentication failed. Reporter not started.");
    return;
  }

  const intervalMs = intervalMinutes * 60_000;
  const dataKeyParts = [service, category, regionId];
  if (workerId && workerId !== "default_worker") dataKeyParts.push(workerId);
  const dataKey = dataKeyParts.join("_");

  log(
    "info",
    `Reporting to ${serverUrl}/report-stats every ${intervalMinutes} min`
  );
  log("info", `Data key: ${dataKey}`);

  async function tick() {
    let attempt = 0;
    while (attempt <= maxRetries) {
      try {
        const stats = await collectStats();
        if (process.env.DEBUG == true)
          log("debug", `Collected stats for ${dataKey}`, stats);
        const res = await fetch(`${serverUrl}/report-stats`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-Secret-Key": currentSecretKey,
          },
          body: JSON.stringify({
            dataKey,
            service,
            category,
            regionId,
            workerId,
            stats,
          }),
        });

        if (res.status === 401) {
          log("warn", "SecretKey expired, re-authenticating...");
          currentSecretKey = await auth(
            serverUrl,
            sharedJwt,
            service,
            category,
            regionId,
            workerId
          );
          attempt++;
          continue;
        }

        if (!res.ok) {
          log("error", `Send failed: ${res.status} ${res.statusText}`);
          throw new Error(await res.text());
        }

        log("info", `Stats sent successfully for ${dataKey}`);
        break;
      } catch (err) {
        attempt++;
        log("error", `Reporting error (attempt ${attempt}):`, err.message);
        if (attempt > maxRetries) {
          log("error", "Max retries reached. Skipping cycle.");
          break;
        }
        log("info", `Retrying in ${retryDelayMs / 1000}s`);
        await new Promise((r) => setTimeout(r, retryDelayMs));
      }
    }
  }

  tick();
  reportingInterval = setInterval(tick, intervalMs);
}

export function stopReporting() {
  if (reportingInterval) {
    clearInterval(reportingInterval);
    reportingInterval = null;
    currentSecretKey = null;
    log("info", "Reporting stopped.");
  } else {
    log("info", "Reporter not running.");
  }
}
