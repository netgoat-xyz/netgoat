import fs from "fs";
import path from "path";
import http from "http";
import https from "https";
import net from "net";
import tls from "tls";
import crypto from "crypto";
import { Elysia } from "elysia";
import { Eta } from "eta";
import { request, Agent, setGlobalDispatcher } from "undici";
import { parse } from "tldts";
import Redis from "ioredis";
import acme from "acme-client";
import { S3Client } from "bun";

// NOTE: These are assumed to be implemented elsewhere in the project
import WAF from "../utils/ruleScript.js";
import domains from "../database/mongodb/schema/domains.js";
import S3Filesystem from "../utils/S3.js";
import logger from "../utils/logger.js";

// --- GLOBAL CONFIGURATION AND CONSTANTS ---

const CERTS_DIR = path.join(process.cwd(), "database", "certs");
const PUBLIC_DIR = path.resolve(process.cwd(), "public"); // Canonical path for security checks

const CACHE_TTL_DOMAIN = 60; // TTL for domain documents (seconds)
const CACHE_TTL_WAF = 3600;  // TTL for WAF rule scripts (seconds)
const DEFAULT_PROXY_TTL = 30; // Default TTL for upstream responses (seconds)

// List of file extensions considered static for caching/local serving
const STATIC_EXTENSIONS = new Set([
  ".js",
  ".css",
  ".ico",
  ".png",
  ".jpg",
  ".jpeg",
  ".svg",
  ".webp",
  ".json",
  ".wasm",
  ".map",
]);

// Initialize directories
if (!fs.existsSync(CERTS_DIR)) {
  logger.info(`Creating certs directory: ${CERTS_DIR}`);
  fs.mkdirSync(CERTS_DIR, { recursive: true });
}

// --- INITIALIZATION ---

const app = new Elysia();
const eta = new Eta({ views: path.join(process.cwd(), "views") });

// Redis client setup
const redis = new Redis(process.env.REDIS_URL);
redis.connect().catch((e) => logger.error("Redis Connection Failed:", e.message));

// S3 Clients and Filesystems
const WAFRulesClient = new S3Client({
  accessKeyId: process.env.MINIO_ACCESS,
  secretAccessKey: process.env.MINIO_SECRET,
  bucket: "waf-rules",
  endpoint: process.env.MINIO_ENDPOINT,
});
const SSLCertsClient = new S3Client({
  accessKeyId: process.env.MINIO_ACCESS,
  secretAccessKey: process.env.MINIO_SECRET,
  bucket: "ssl-certs",
  endpoint: process.env.MINIO_ENDPOINT,
});

const WAF_FS = new S3Filesystem(WAFRulesClient, "waf_cache");
const SSL_FS = new S3Filesystem(SSLCertsClient, "ssl_cache");
const waf = new WAF();

// Undici Agents for performance
const CLICKHOUSE_URL = process.env.CLICKHOUSE_URL || "http://localhost:8123";
const CLICKHOUSE_USER = process.env.CLICKHOUSE_USER || "default";
const CLICKHOUSE_PASSWORD = process.env.CLICKHOUSE_PASSWORD || "";
const undiciAgent = new Agent({ connections: 100, pipelining: 1 });
const clickhouseAgent = new Agent({ connections: 10, pipelining: 1 });

setGlobalDispatcher(undiciAgent);
http.globalAgent.keepAlive = true;
https.globalAgent.keepAlive = true;

// --- IN-MEMORY CACHES ---

// Cache for Domain Documents (used in proxy logic)
const domainMemoryCache = new Map();
// Cache for resolved SecureContexts (used in SNICallback)
const sniCertCache = new Map();
// Cache for SNI Domain documents (separate from main domain cache for TLS lookups)
const sniDomainCache = new Map();

// --- UTILITIES ---

/**
 * Extracts the client's originating IP address from headers, prioritizing
 * trusted sources (X-Forwarded-For, X-Real-IP) before falling back to socket.
 * @param {http.IncomingMessage} req
 * @returns {string} Client IP address.
 */
function getClientIp(req) {
  const xff = req.headers["x-forwarded-for"];
  if (xff) return xff.split(",")[0].trim();
  return req.headers["x-real-ip"] || req.socket.remoteAddress || "unknown";
}

/**
 * Formats a Date object into the ISO 8601 string format required by ClickHouse (including milliseconds).
 * @param {Date} dt
 * @returns {string} ClickHouse compatible timestamp.
 */
function formatCHDate(dt) {
  const pad = (n, z = 2) => n.toString().padStart(z, "0");
  return `${dt.getUTCFullYear()}-${pad(dt.getUTCMonth()+1)}-${pad(dt.getUTCDate())} ` +
         `${pad(dt.getUTCHours())}:${pad(dt.getUTCMinutes())}:${pad(dt.getUTCSeconds())}.${pad(dt.getUTCMilliseconds(),3)}`;
}

// --- ACME & TLS MANAGEMENT ---

async function loadOrCreateAccountKey() {
  const ACCOUNT_KEY_PATH = path.join(CERTS_DIR, "acme-account.key");
  if (fs.existsSync(ACCOUNT_KEY_PATH))
    return fs.readFileSync(ACCOUNT_KEY_PATH, "utf8");
  const key = await acme.openssl.createPrivateKey();
  fs.writeFileSync(ACCOUNT_KEY_PATH, key, { mode: 0o600 });
  return key;
}

async function getAcmeClient() {
  const accountKey = await loadOrCreateAccountKey();
  return new acme.Client({
    directoryUrl: acme.directory.letsencrypt.production,
    accountKey,
  });
}

/**
 * SSE Optimization: Centralized Domain Data Fetching
 * Uses a two-tier caching strategy (in-memory Map then Redis) for high efficiency.
 * @param {string} domain The full hostname to look up.
 * @returns {Promise<object | null>} The domain document.
 */
async function getDomainData(domain) {
  if (domainMemoryCache.has(domain)) return domainMemoryCache.get(domain);
  const cacheKey = `domain:${domain}`;
  const cached = await redis.get(cacheKey);
  
  if (cached) {
    const doc = JSON.parse(cached);
    domainMemoryCache.set(domain, doc);
    return doc;
  }
  
  // Use .limit(1) for efficiency as we only need one result
  const doc = await domains.findOne({ domain }).lean().limit(1); 

  if (doc) {
    redis.setex(cacheKey, CACHE_TTL_DOMAIN, JSON.stringify(doc)).catch(logger.error);
    domainMemoryCache.set(domain, doc);
  }
  return doc;
}

/**
 * Optimized Certificate Retrieval for SNICallback
 * Checks in-memory cache, then S3, and verifies expiration.
 * @param {string} userId Owner ID of the domain.
 * @param {string} domain The domain name.
 * @param {string} subdomain Subdomain slug ('@' for root).
 * @returns {Promise<{cert: string, key: string} | null>} Certificate and key data.
 */
async function ensureCertificateForUserDomain(userId, domain, subdomain = "@") {
  const cacheKey = `${userId}/${domain}/${subdomain}`;

  // 1. Check in-memory SNI cache
  if (sniCertCache.has(cacheKey)) {
    const cached = sniCertCache.get(cacheKey);
    // Check if the cached cert is valid for more than 14 days
    if (new Date(cached.notAfter).getTime() - Date.now() > 1000 * 60 * 60 * 24 * 14) {
      return { cert: cached.cert, key: cached.key };
    }
    // If expired soon, delete the cached item and proceed to re-fetch/renewal logic.
    sniCertCache.delete(cacheKey);
  }

  const certKey = `${cacheKey}/fullchain.pem`;
  const privKeyKey = `${cacheKey}/privkey.pem`;

  // 2. Check S3/File Cache
  const [cachedCert, cachedKey] = await Promise.all([
    SSL_FS.read(certKey),
    SSL_FS.read(privKeyKey),
  ]);

  if (cachedCert && cachedKey) {
    try {
      const certData = cachedCert.content.toString();
      const keyData = cachedKey.content.toString();

      const info = acme.openssl.readCertificateInfo(certData);
      
      // Update the in-memory cache and return if valid
      sniCertCache.set(cacheKey, { ...info, cert: certData, key: keyData });

      if (new Date(info.notAfter).getTime() - Date.now() > 1000 * 60 * 60 * 24 * 14) {
        return { cert: certData, key: keyData };
      }
    } catch (e) {
      logger.error("Certificate validation failed during S3 fetch:", e.message);
    }
  }

  // 3. Renewal Logic (Placeholder)
  // CRITICAL: In a real system, this is where the ACME client would be used to 
  // request/renew the certificate if it's missing or expired.
  logger.warn(`[ACME] Certificate renewal required for: ${domain}`);
  // const client = await getAcmeClient();
  // await client.createOrder... (renewal logic here)

  return null; // Return null if fetching/renewal fails
}

// --- WAF & CACHING LOGIC ---

/**
 * Fetches the domain-specific WAF rules, using a two-tier cache (Redis then S3/File cache).
 * @param {string} domain The root domain name.
 * @param {string} slug The subdomain slug ('@' for root).
 * @returns {Promise<string>} The rule script content (JavaScript code).
 */
async function getCustomWafRules(domain, slug) {
  const effectiveSlug = slug || "@";
  const cacheKey = `waf:rules:${domain}_${effectiveSlug}`;
  const s3Key = `custom-rules/${domain}/${effectiveSlug}/consolidated.js`;

  const cachedRules = await redis.get(cacheKey);
  if (cachedRules) {
    logger.waf(`[RULES] Cache hit for ${cacheKey}`);
    return cachedRules;
  }

  try {
    const fileData = await WAF_FS.read(s3Key);

    if (fileData && fileData.content) {
      const ruleContent = fileData.content.toString();
      // Use CACHE_TTL_WAF (3600s) for slow-changing WAF rules
      await redis.setex(cacheKey, CACHE_TTL_WAF, ruleContent); 
      logger.waf(`[RULES] S3 fetch success for ${s3Key}. Caching.`);
      return ruleContent;
    }

    // Cache an empty string if no rules found to prevent repeated S3 lookups
    await redis.setex(cacheKey, CACHE_TTL_WAF, "");
    return "";
  } catch (err) {
    logger.error(
      `[WAF RULES] Failed to fetch rules for ${domain}/${effectiveSlug}:`,
      err.message
    );
    // Cache empty string on error, but with a shorter TTL (e.g., 60s) for recovery
    await redis.setex(cacheKey, CACHE_TTL_DOMAIN, ""); 
    return "";
  }
}

function cacheResponse(key, value, ttl = DEFAULT_PROXY_TTL) {
  redis.setex(key, ttl, value).catch(logger.error);
}

async function getCachedResponse(key) {
  return redis.get(key);
}

/**
 * Determines the subdomain slug being requested.
 * @param {http.IncomingMessage} req
 * @param {string} domain The root domain name.
 * @returns {string} The slug ('@' for root domain, or the subdomain part).
 */
function getSubdomainSlug(req, domain) {
  const host = (req.headers.host || domain).split(":")[0];
  if (host === domain) return "@";
  
  // Calculate the index where the root domain starts in the hostname
  const rootIndex = host.indexOf(`.${domain}`);
  if (rootIndex > 0) {
    return host.substring(0, rootIndex);
  }
  return "@";
}

/**
 * Handles the WAF inspection and any resulting security action (block, redirect, challenge).
 * @param {http.IncomingMessage} req
 * @param {http.ServerResponse} res
 * @param {string} effectiveDomain The root domain being targeted.
 * @returns {Promise<{handled: boolean, statusCode: number, cacheStatus: string}>}
 */
async function handleWafAndChallenge(req, res, effectiveDomain) {
  const subdomainSlug = getSubdomainSlug(req, effectiveDomain);
  
  // SSE Optimization: Fetch WAF rules concurrently with other high-level lookups
  const customRulesCode = await getCustomWafRules(effectiveDomain, subdomainSlug);

  const wafReq = {
    method: req.method,
    url: req.url,
    headers: req.headers,
    ip: getClientIp(req),
    body: null, // Body parsing is omitted here for simplicity
  };

  try {
    const result = await waf.checkRequestWithCode(
      wafReq,
      customRulesCode,
      effectiveDomain
    );

    logger.waf(`[FLOW] WAF Result for ${req.url}: ${result.action}`);

    if (result.action === "allow") {
      return { handled: false, statusCode: 200, cacheStatus: "PASS" };
    }

    if (result.action === "block") {
      let htmlBody = await eta.render("error/waf.ejs", { reason: "blocked" });
      if (!res.headersSent) {
        res.writeHead(403, { "Content-Type": "text/html" });
        res.end(htmlBody);
      }
      return { handled: true, statusCode: 403, cacheStatus: "BLOCKED" };
    }

    if (result.action === "redirect") {
      if (!res.headersSent) {
        res.writeHead(302, { Location: result.url });
        res.end();
      }
      return { handled: true, statusCode: 302, cacheStatus: "REDIRECT" };
    }

    if (result.action === "challenge") {
      const token = crypto.randomUUID();
      redis.setex(`challenge:${token}`, 300, JSON.stringify({ ip: wafReq.ip, type: result.type || "basic" }))
        .catch(logger.error);

      let htmlBody = await eta.render("challenge.eta", { token });
      if (!res.headersSent) {
        res.writeHead(403, { "Content-Type": "text/html" });
        res.end(htmlBody);
      }
      return { handled: true, statusCode: 403, cacheStatus: "CHALLENGE" };
    }

  } catch (err) {
    logger.error(`[WAF ERROR] Uncaught exception during WAF execution for ${effectiveDomain}:`, err.message);
    // Fail open on WAF error (allow the request to proceed)
    return { handled: false, statusCode: 500, cacheStatus: "WAF_ERROR" };
  }
}

// --- LOGGING ---

/**
 * Aggregates and sends final request metrics to ClickHouse.
 * @param {object} req - The incoming HTTP request object.
 * @param {number} statusCode - The final HTTP status code sent to the client.
 * @param {string} cacheStatus - The cache status ('HIT', 'MISS', 'PASS', 'BLOCKED', etc.).
 * @param {bigint} startTime - The process.hrtime.bigint() timestamp when the request started.
 * @param {string} traceId - The unique trace ID for the request.
 */
export async function logRequestMetrics(req, statusCode, cacheStatus, startTime, traceId) {
  try {
    const endTime = process.hrtime.bigint();
    const durationMs = Number(endTime - startTime) / 1e6;

    const host = (req.headers.host || "").split(":")[0];
    const clientIp = getClientIp(req);

    const logEntry = {
      timestamp: formatCHDate(new Date()),
      method: req.method,
      host,
      path: req.url,
      ip: clientIp,
      status: statusCode,
      cache: cacheStatus,
      duration_ms: parseFloat(durationMs.toFixed(2)),
      user_agent: req.headers["user-agent"] || "",
      referer: req.headers.referer || "",
      trace_id: traceId,
    };

    let url = `${CLICKHOUSE_URL}/?query=INSERT%20INTO%20netgoat.request_logs%20FORMAT%20JSONEachRow`;
    if (CLICKHOUSE_USER) {
      url += `&user=${encodeURIComponent(CLICKHOUSE_USER)}`;
      if (CLICKHOUSE_PASSWORD) url += `&password=${encodeURIComponent(CLICKHOUSE_PASSWORD)}`;
    }

    // CRITICAL: Format as a single line JSON with a trailing newline for JSONEachRow format
    const jsonLine = JSON.stringify(logEntry) + "\n";
    
    // SSE Optimization: Send log entry asynchronously without blocking main proxy loop
    const chRes = await request(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: jsonLine,
      dispatcher: clickhouseAgent,
      timeout: 3000,
    });

    if (chRes.statusCode !== 200) {
      const errorText = await chRes.body.text();
      logger.error(`ClickHouse insert failed (${chRes.statusCode}): ${errorText}`);
    } else {
      logger.debug(`Log inserted to ClickHouse: ${req.method} ${req.url} (${statusCode})`);
    }
  } catch (err) {
    logger.warn(`ClickHouse connection error (log skipped): ${err.message}`);
  }
}

// --- CORE PROXY LOGIC ---

/**
 * Resolves the requested domain, finds the correct target service based on the subdomain/slug,
 * and handles Banned IP checks.
 * @param {string} host The incoming hostname.
 * @returns {Promise<{domainData: object, targetService: object, effectiveDomain: string, requestedSlug: string} | null>}
 */
async function resolveHostAndTarget(host) {
  let domainData = await getDomainData(host);
  let effectiveDomain = host;
  let requestedSlug = "";

  // If direct lookup fails, attempt root domain lookup using TLD extraction
  if (!domainData) {
    const { domain: tldtsDomain } = parse(host);
    if (tldtsDomain && tldtsDomain !== host) {
      domainData = await getDomainData(tldtsDomain);
      effectiveDomain = tldtsDomain;
    }
  }

  if (!domainData) return null;

  // Determine the requested slug/subdomain
  if (host !== domainData.domain) {
    const domainPrefix = "." + domainData.domain;
    if (host.endsWith(domainPrefix)) {
      requestedSlug = host.slice(0, host.length - domainPrefix.length);
    }
  }
  
  // Find the target service configuration
  const targetService = domainData.proxied?.find(
    (p) => p.slug === requestedSlug || (p.slug === "@" && requestedSlug === "")
  );

  if (!targetService) return null;

  // Check Banned IP List (Early Exit)
  const ip = getClientIp(req);
  if (targetService.SeperateBannedIP?.some((b) => b.ip === ip)) {
    return {
      error: { 
        statusCode: 403, 
        cacheStatus: "BANNED_IP", 
        message: "Forbidden (Banned IP)" 
      }
    };
  }

  return { domainData, targetService, effectiveDomain, requestedSlug };
}

/**
 * Handles proxying the request to the upstream target service.
 * @param {http.IncomingMessage} req
 * @param {http.ServerResponse} res
 * @param {object} targetService The service configuration.
 * @param {string} cacheKey Redis cache key.
 * @param {boolean} shouldCache Whether the response should be cached.
 * @param {string} traceId Request trace ID.
 * @returns {Promise<{finalStatusCode: number, cacheStatus: string}>}
 */
async function handleUpstreamProxy(req, res, targetService, cacheKey, shouldCache, traceId) {
    const rawUrl = req.url || "/";
    const method = req.method;
    const protocol = targetService.SSL ? "https" : "http";
    const targetUrl = `${protocol}://${targetService.ip}:${targetService.port}${rawUrl}`;

    const headers = { ...req.headers };
    // Clean up hop-by-hop headers
    delete headers.connection;
    delete headers["keep-alive"];
    delete headers["transfer-encoding"];
    delete headers["accept-encoding"];
    headers["x-tracelet-id"] = traceId;

    const upstream = await request(targetUrl, {
      method,
      headers,
      body: method === "GET" ? undefined : req,
      dispatcher: undiciAgent,
    });

    const finalStatusCode = upstream.statusCode;
    const cacheStatus = "MISS";

    // 1. Prepare response headers
    const outHeaders = {};
    for (const [k, v] of Object.entries(upstream.headers || {})) {
      const lowerK = k.toLowerCase();
      // Filter out headers that should not be forwarded to the client
      if (
        lowerK !== "content-length" &&
        lowerK !== "transfer-encoding" &&
        lowerK !== "content-encoding"
      )
        outHeaders[k] = v;
    }
    outHeaders["x-powered-by"] = "NetGoat";
    outHeaders["x-tracelet-id"] = traceId;

    if (shouldCache && finalStatusCode === 200) {
      // 2. Caching (Write) - Read full body, cache, and respond
      const bodyBuf = await upstream.body.arrayBuffer();
      const bodyText = Buffer.from(bodyBuf).toString();
      cacheResponse(cacheKey, bodyText, targetService.cacheTTL || DEFAULT_PROXY_TTL);
      outHeaders["content-length"] = Buffer.byteLength(bodyText);
      if (!res.headersSent) {
        res.writeHead(finalStatusCode, outHeaders);
        res.end(bodyText);
      }
    } else {
      // 3. Streaming - Pipe the body directly
      if (!res.headersSent) {
        res.writeHead(finalStatusCode, outHeaders);
        upstream.body.pipe(res);
      }
    }
    
    return { finalStatusCode, cacheStatus };
}


/**
 * Primary request handler for both HTTP and HTTPS.
 * Implements the full request pipeline: Trace -> Security -> Cache -> Proxy.
 */
async function proxyHttp(req, res) {
  const startTime = process.hrtime.bigint();
  const traceId = crypto.randomUUID(); 
  let finalStatusCode = 500;
  let cacheStatus = "ERROR";

  res.setHeader("x-tracelet-id", traceId);

  try {
    const rawUrl = req.url || "/";
    const host = (req.headers.host || "").split(":")[0];
    const method = req.method;
    const urlPath = decodeURIComponent(rawUrl.split("?")[0] || "/");
    const ext = path.extname(urlPath).toLowerCase();

    if (!host) {
      finalStatusCode = 400;
      if (!res.headersSent) res.writeHead(400).end("Missing Host");
      return;
    }

    // 1. ACME Challenge Handling (Priority 1)
    if (rawUrl.startsWith("/.well-known/acme-challenge/")) {
      const token = rawUrl.split("/").pop();
      const keyAuth = await redis.get(`acme:http:${token}`);
      if (keyAuth) {
        finalStatusCode = 200;
        cacheStatus = "ACME";
        if (!res.headersSent) res.writeHead(200, { "Content-Type": "text/plain" }).end(keyAuth);
        return;
      }
    }

    // 2. Static File Handling (Priority 2) - Includes Path Traversal Check
    const isStaticFile = method === "GET" && 
      (urlPath === "/favicon.ico" || 
       urlPath.startsWith("/_next/static/") || 
       urlPath.startsWith("/static/") ||
       STATIC_EXTENSIONS.has(ext));

    if (isStaticFile) {
      // SSE Security Fix: Path Traversal Prevention
      const requestedPath = path.join(PUBLIC_DIR, urlPath.replace(/^\//, ""));
      const filePath = path.resolve(requestedPath);

      if (!filePath.startsWith(PUBLIC_DIR)) {
        logger.warn(`[FS] Path Traversal attempt detected: ${urlPath}`);
        finalStatusCode = 404;
        if (!res.headersSent) res.writeHead(404).end("File not found");
        return;
      }
      
      if (fs.existsSync(filePath)) {
        finalStatusCode = 200;
        cacheStatus = "LOCAL-FS";
        const stats = fs.statSync(filePath);
        if (!res.headersSent) {
          res.writeHead(200, {
            "Content-Length": String(stats.size),
            "Content-Type": ext === ".js" ? "application/javascript" : ext === ".css" ? "text/css" : "application/octet-stream",
            "Cache-Control": urlPath.startsWith("/_next/static/") ? "public,max-age=31536000,immutable" : "public,max-age=60",
            "x-cache": "LOCAL-FS",
          });
          fs.createReadStream(filePath).pipe(res);
        }
        return;
      }
    }

    // 3. Domain Resolution & Target Lookup
    const resolution = await resolveHostAndTarget(host);

    if (resolution?.error) {
        finalStatusCode = resolution.error.statusCode;
        cacheStatus = resolution.error.cacheStatus;
        if (!res.headersSent) res.writeHead(finalStatusCode).end(resolution.error.message);
        return;
    }
    
    if (!resolution) {
      finalStatusCode = 404;
      if (!res.headersSent) res.writeHead(404).end("Domain not configured");
      return;
    }
    
    const { targetService, effectiveDomain } = resolution;

    // 4. WAF and Challenge Handling (Priority 3)
    const wafResult = await handleWafAndChallenge(req, res, effectiveDomain);
    if (wafResult.handled) {
      finalStatusCode = wafResult.statusCode;
      cacheStatus = wafResult.cacheStatus;
      return;
    }
    // Note: If WAF fails, we continue with 'WAF_ERROR' cacheStatus.

    // 5. Caching (Read)
    const cacheKey = `resp:${host}:${rawUrl}`;
    const shouldCache = method === "GET" && targetService.cacheable;

    if (shouldCache) {
      const cached = await getCachedResponse(cacheKey);
      if (cached) {
        finalStatusCode = 200;
        cacheStatus = "HIT";
        if (!res.headersSent) {
          res.writeHead(200, { "Content-Type": "text/html", "x-cache": "HIT" });
          res.end(cached);
        }
        return;
      }
    }

    // 6. Upstream Proxy Request & Response
    const proxyResult = await handleUpstreamProxy(req, res, targetService, cacheKey, shouldCache, traceId);
    finalStatusCode = proxyResult.finalStatusCode;
    cacheStatus = proxyResult.cacheStatus;

  } catch (err) {
    logger.error("Proxy Error:", err.message || String(err));
    // Set 500 if an error occurred before status code was explicitly set
    finalStatusCode = finalStatusCode >= 500 ? 500 : finalStatusCode; 
    cacheStatus = "ERROR";
    
    if (!res.headersSent) {
      let html = await eta.render("error/500.ejs", { error: err.message || String(err) });
      res.writeHead(finalStatusCode, { "Content-Type": "text/html" });
      res.end(html || "<h1>500 Internal Server Error</h1>");
    }
  } finally {
    // 7. Final Logging (Async and non-blocking)
    // SSE Optimization: No await here to ensure the client response is prioritized
    logRequestMetrics(req, finalStatusCode, cacheStatus, startTime, traceId);
  }
}

/**
 * Handles WebSocket/Upgrade requests (HTTP 101 Switching Protocols).
 * @param {http.IncomingMessage} req
 * @param {net.Socket} socket
 * @param {Buffer} head
 */
async function handleUpgrade(req, socket, head) {
  const host = (req.headers.host || "").split(":")[0];
  logger.info(`[WS] Attempting upgrade for: ${host}${req.url}`);

  try {
    const resolution = await resolveHostAndTarget(host);
    
    if (!resolution || resolution.error) {
      logger.warn(`[WS] Domain not configured or blocked: ${host}`);
      socket.destroy();
      return;
    }
    
    const { targetService } = resolution;

    if (!targetService.WS) {
      logger.warn(`[WS] No WS service configured for: ${host}`);
      socket.destroy();
      return;
    }

    // Establish connection to the upstream server
    const targetSocket = net.connect(
      targetService.port,
      targetService.ip,
      () => {
        // Send the original handshake request headers to the target
        targetSocket.write(head);
        // Tunnel traffic bidirectionally
        socket.pipe(targetSocket).pipe(socket);
        logger.info(`[WS] Connection established for: ${host}${req.url}`);
      }
    );

    // Error handling to prevent resource leakage
    targetSocket.on("error", (err) => {
      logger.error(`[WS] Target socket error for ${host}: ${err.message}`);
      socket.destroy();
    });

    socket.on("error", (err) => {
      logger.error(`[WS] Client socket error for ${host}: ${err.message}`);
      targetSocket.destroy();
    });

    socket.on("end", () => logger.info(`[WS] Client disconnected: ${host}`));
  } catch (e) {
    logger.error(
      `[WS] Uncaught error in upgrade handler for ${host}: ${e.message}`
    );
    socket.destroy();
  }
}

async function start() {
  // --- HTTP Server (Port 80) ---
  const httpServer = http.createServer(proxyHttp);
  httpServer.timeout = 30000;
  httpServer.on("upgrade", handleUpgrade);
  httpServer.listen(80, () => logger.success("Reverse Proxy active (80)"));

  // --- HTTPS Server (Port 443) Setup ---
  const defaultCert = fs.existsSync(path.join(CERTS_DIR, "default.pem")) ? fs.readFileSync(path.join(CERTS_DIR, "default.pem")) : null;
  const defaultKey = fs.existsSync(path.join(CERTS_DIR, "default.key")) ? fs.readFileSync(path.join(CERTS_DIR, "default.key")) : null;

  const options =
    defaultCert && defaultKey
      ? {
          key: defaultKey,
          cert: defaultCert,
          // SSE Optimization: SNI Callback uses in-memory/S3 cache to avoid MongoDB lookup on every connection.
          SNICallback: async (servername, cb) => {
            try {
              // 1. Check in-memory SNI domain cache first
              let domainDoc = sniDomainCache.get(servername);
              if (!domainDoc) {
                // 2. Fallback to MongoDB/Redis if not in SNI cache
                domainDoc = await domains.findOne({ domain: servername }).lean().limit(1);
                if (domainDoc) {
                    sniDomainCache.set(servername, domainDoc);
                }
              }

              if (!domainDoc) return cb(new Error(`Domain not found: ${servername}`));
              
              const certData = await ensureCertificateForUserDomain(
                domainDoc.ownerId,
                servername
              );

              if (certData) {
                 cb(null, tls.createSecureContext(certData));
              } else {
                 // Fall back to default context if custom cert is missing/expired
                 cb(null, tls.createSecureContext({ key: defaultKey, cert: defaultCert }));
              }
            } catch (e) {
              logger.error(`SNI Failure for ${servername}:`, e.message);
              // Fail open to default cert or error
              cb(e); 
            }
          },
        }
      : {
          // Fallback dummy certs for environments without real ones
          key: fs.readFileSync(path.join(CERTS_DIR, "dummy.key")),
          cert: fs.readFileSync(path.join(CERTS_DIR, "dummy.crt")),
        };

  const httpsServer = https.createServer(options, proxyHttp);
  httpsServer.timeout = 30000;
  httpsServer.on("upgrade", handleUpgrade);
  httpsServer.listen(443, () => logger.success("Reverse Proxy active (443)"));
}

start();