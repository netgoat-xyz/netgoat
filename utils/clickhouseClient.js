import { request } from "undici";
import logger from "./logger.js";

const CLICKHOUSE_URL = process.env.CLICKHOUSE_URL || "http://localhost:8123";
const CLICKHOUSE_DB = process.env.CLICKHOUSE_DB || "netgoat";
const CLICKHOUSE_USER = process.env.CLICKHOUSE_USER || "default";
const CLICKHOUSE_PASSWORD = process.env.CLICKHOUSE_PASSWORD || "";

/**
 * Execute a ClickHouse query
 */
export async function executeQuery(sql, format = "JSON") {
  try {
    let url = `${CLICKHOUSE_URL}/?database=${CLICKHOUSE_DB}&default_format=${format}`;
    if (CLICKHOUSE_USER) {
      url += `&user=${CLICKHOUSE_USER}`;
    }
    if (CLICKHOUSE_PASSWORD) {
      url += `&password=${CLICKHOUSE_PASSWORD}`;
    }

    const res = await request(url, {
      method: "POST",
      headers: {
        "Content-Type": "text/plain",
      },
      body: sql,
      timeout: 5000,
    });

    if (res.statusCode !== 200) {
      const text = await res.body.text();
      throw new Error(`ClickHouse error (${res.statusCode}): ${text}`);
    }

    const text = await res.body.text();
    if (!text || text.trim() === "") {
      return format === "JSON" ? { data: [] } : "";
    }
    return format === "JSON" ? JSON.parse(text) : text;
  } catch (err) {
    logger.error("ClickHouse query error:", err.message);
    throw err;
  }
}

/**
 * Insert logs using native format (optimized)
 */
export async function insertLogs(logs) {
  try {
    if (!logs || logs.length === 0) return;

    // Don't block on ClickHouse availability (fire-and-forget)
    const available = await isClickHouseAvailable();
    if (!available) {
      logger.debug("ClickHouse unavailable, skipping log insert");
      return;
    }

    const url = `${CLICKHOUSE_URL}/?database=${CLICKHOUSE_DB}`;
    const jsonl = logs.map((log) => JSON.stringify(log)).join("\n") + "\n";

    const res = await request(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-ndjson",
      },
      body: `INSERT INTO ${CLICKHOUSE_DB}.request_logs FORMAT JSONEachRow\n${jsonl}`,
      timeout: 5000,
    });

    if (res.statusCode !== 200) {
      const text = await res.body.text();
      throw new Error(`Failed to insert logs: ${text}`);
    }

    logger.debug(`Inserted ${logs.length} logs to ClickHouse`);
  } catch (err) {
    logger.debug("ClickHouse insert error:", err.message);
    // Don't fail - ClickHouse might come up later
  }
}

/**
 * Check if ClickHouse is available
 */
export async function isClickHouseAvailable() {
  try {
    const res = await request(`${CLICKHOUSE_URL}/ping`, { timeout: 3000 });
    return res.statusCode === 200;
  } catch (err) {
    return false;
  }
}

/**
 * Initialize the ClickHouse database and table
 */
export async function initializeClickHouse() {
  try {
    logger.info("Initializing ClickHouse...");

    // Check if ClickHouse is available
    const available = await isClickHouseAvailable();
    if (!available) {
      logger.warn(
        "ClickHouse not available at",
        CLICKHOUSE_URL,
        "- skipping initialization. Will retry later."
      );
      return false;
    }

    // Create database
    await executeQuery(`CREATE DATABASE IF NOT EXISTS ${CLICKHOUSE_DB}`);
    logger.success(`Database ${CLICKHOUSE_DB} ready`);

    // Create request_logs table
    const createTableSQL = `
      CREATE TABLE IF NOT EXISTS ${CLICKHOUSE_DB}.request_logs (
        timestamp DateTime,
        method String,
        host String,
        path String,
        ip String,
        status UInt16,
        cache String,
        duration_ms Float32,
        user_agent String,
        referer String,
        trace_id String
      )
      ENGINE = MergeTree()
      ORDER BY (timestamp, host)
      PARTITION BY toYYYYMM(timestamp)
    `;

    await executeQuery(createTableSQL);
    logger.success(`Table request_logs ready`);
    return true;
  } catch (err) {
    logger.warn("ClickHouse initialization warning:", err.message);
    return false;
  }
}

/**
 * Query logs with filtering
 */
export async function queryLogs(filters = {}) {
  try {
    if (!(await isClickHouseAvailable())) return [];

    const {
      limit = 100,
      domain = null,
      startDate = null,
      endDate = null,
      range = null,
    } = filters;

    let sql = `SELECT timestamp, method, host, path, ip, status, cache, duration_ms, user_agent, referer, trace_id 
               FROM ${CLICKHOUSE_DB}.request_logs WHERE 1=1`;

    if (domain) sql += ` AND host = '${domain.replace(/'/g, "''")}'`;

    const ranges = {
      "90d": "INTERVAL 90 DAY",
      "30d": "INTERVAL 30 DAY",
      "24h": "INTERVAL 24 HOUR",
      "1h": "INTERVAL 1 HOUR",
    };

    const interval = ranges[range] || null;
    if (interval) sql += ` AND timestamp >= now() - ${interval}`;

    if (startDate) sql += ` AND timestamp >= '${startDate}'`;
    if (endDate) sql += ` AND timestamp <= '${endDate}'`;

    sql += ` ORDER BY timestamp DESC LIMIT ${Math.min(limit, 10000)}`;

    const result = await executeQuery(sql);
    return result.data || [];
  } catch {
    return [];
  }
}

/**
 * Get log statistics
 */
export async function getLogStats(domain = null, range = null) {
  try {
    // Check if ClickHouse is available
    if (!(await isClickHouseAvailable())) {
      logger.warn("ClickHouse unavailable, returning empty stats");
      return [];
    }

    const ranges = {
      "90d": "INTERVAL 90 DAY",
      "30d": "INTERVAL 30 DAY",
      "7d": "INTERVAL 7 DAY",
      "24h": "INTERVAL 24 HOUR",
      "1h": "INTERVAL 1 HOUR",
    };

    const interval = ranges[range] || null;

    let sql = `
      SELECT 
        host,
        COUNT() as total,
        avg(duration_ms) as avg_duration,
        MAX(duration_ms) as max_duration,
        countIf(status >= 200 AND status < 300) as success_2xx,
        countIf(status >= 300 AND status < 400) as redirect_3xx,
        countIf(status >= 400 AND status < 500) as client_error_4xx,
        countIf(status >= 500) as server_error_5xx
      FROM ${CLICKHOUSE_DB}.request_logs
      WHERE 1=1
    `;

    if (interval) sql += ` AND timestamp >= now() - ${interval}`;
    if (domain) sql += ` AND host = '${domain.replace(/'/g, "''")}'`;

    sql += ` GROUP BY host ORDER BY total DESC`;

    const result = await executeQuery(sql);
    return result.data || [];
  } catch (err) {
    logger.warn("Failed to get stats:", err.message);
    return [];
  }
}

export default {
  executeQuery,
  insertLogs,
  initializeClickHouse,
  queryLogs,
  getLogStats,
};
