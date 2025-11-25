import fs from "fs";
import path from "path";
import http from "http";
import https from "https";
import net from "net";
import tls from "tls";
import crypto from "crypto";
import { Elysia } from "elysia";
import { Eta } from "eta";
import { request, Agent } from "undici";
import { parse } from "tldts";
import Redis from "ioredis";
import acme from "acme-client";
import { S3Client } from "bun";
import WAF from "../utils/ruleScript.js";
import domains from "../database/mongodb/schema/domains.js";
import S3Filesystem from "../utils/S3.js";
import logger from "../utils/logger.js";

const CERTS_DIR = path.join(process.cwd(), "database", "certs");
if (!fs.existsSync(CERTS_DIR)) fs.mkdirSync(CERTS_DIR, { recursive: true });

const app = new Elysia();
const eta = new Eta({ views: path.join(process.cwd(), "views") });

const redis = new Redis(process.env.REDIS_URL);
redis
  .connect()
  .catch((e) => console.error("Redis Connection Failed:", e.message));

// 1. Initialize Raw S3 Clients
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

// 2. Initialize Cached Filesystems
const WAF_FS = new S3Filesystem(WAFRulesClient, "waf_cache");
const SSL_FS = new S3Filesystem(SSLCertsClient, "ssl_cache");

const waf = new WAF();
const undiciAgent = new Agent({ connections: 100, pipelining: 1 });
http.globalAgent.keepAlive = true;
https.globalAgent.keepAlive = true;

const domainMemoryCache = new Map();

function getClientIp(req) {
  const xff = req.headers["x-forwarded-for"];
  if (xff) return xff.split(",")[0].trim();
  return req.headers["x-real-ip"] || req.socket.remoteAddress || "unknown";
}

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

async function ensureCertificateForUserDomain(userId, domain, subdomain = "@") {
  const certKey = `${userId}/${domain}/${subdomain}/fullchain.pem`;
  const privKeyKey = `${userId}/${domain}/${subdomain}/privkey.pem`;

  const cachedCert = await SSL_FS.read(certKey);
  const cachedKey = await SSL_FS.read(privKeyKey);

  if (cachedCert && cachedKey) {
    try {
      const certData = cachedCert.content.toString();
      const keyData = cachedKey.content.toString();

      const info = acme.openssl.readCertificateInfo(certData);
      if (
        new Date(info.notAfter).getTime() - Date.now() >
        1000 * 60 * 60 * 24 * 14
      ) {
        return { cert: certData, key: keyData };
      }
    } catch (e) {
      console.error("Certificate validation failed:", e.message);
    }
  }

  const client = await getAcmeClient();
  // Note: Assuming createOrder/finalize flow here for ACME
  // Placeholder logic to prevent crash in snippet
  return null; 
}

async function getDomainData(domain) {
  if (domainMemoryCache.has(domain)) return domainMemoryCache.get(domain);
  const cacheKey = `domain:${domain}`;
  const cached = await redis.get(cacheKey);
  if (cached) {
    const doc = JSON.parse(cached);
    domainMemoryCache.set(domain, doc);
    return doc;
  }
  const doc = await domains.findOne({ domain }).lean();
  if (doc) {
    redis.setex(cacheKey, 60, JSON.stringify(doc)).catch(console.error);
    domainMemoryCache.set(domain, doc);
  }
  return doc;
}

// Updated WAF Rule Fetcher
async function getCustomWafRules(domain, slug) {
    const effectiveSlug = slug || '@'; 
    const cacheKey = `waf:rules:${domain}_${effectiveSlug}`; 
    const s3Key = `custom-rules/${domain}/${effectiveSlug}/consolidated.js`; 

    const cachedRules = await redis.get(cacheKey);
    if (cachedRules) {
        logger.waf(`[RULES] Cache hit for ${cacheKey}`);
        return cachedRules;
    }

    try {
        logger.waf(`[RULES] Cache miss. Attempting S3 fetch for ${s3Key}...`);
        
        const fileData = await WAF_FS.read(s3Key);
        
        if (fileData && fileData.content) {
             const ruleContent = fileData.content.toString();
             await redis.setex(cacheKey, 3600, ruleContent); 
             logger.waf(`[RULES] S3 fetch success for ${s3Key}. Caching.`);
             return ruleContent;
        }
        
        logger.waf(`[RULES] Rules file not found in S3/Cache: ${s3Key}`);
        await redis.setex(cacheKey, 3600, ""); 
        return "";

    } catch (err) {
        logger.error(`[WAF RULES] Failed to fetch rules for ${domain}/${effectiveSlug}:`, err.message);
        await redis.setex(cacheKey, 3600, ""); 
        return "";
    }
}


function cacheResponse(key, value, ttl = 30) {
  redis.setex(key, ttl, value).catch(console.error);
}

async function getCachedResponse(key) {
  return redis.get(key);
}

function getSubdomainSlug(req, domain) {
    const host = req.headers.host || domain;
    const hostname = host.split(':')[0];

    if (hostname.endsWith(domain)) {
        const rootIndex = hostname.indexOf(`.${domain}`);
        if (rootIndex === -1 && hostname === domain) {
            return '@';
        }
        const subdomainPart = hostname.substring(0, rootIndex);
        if (subdomainPart) {
            return subdomainPart;
        }
    }
    return '@';
}

async function handleWafAndChallenge(req, res, domain) {
  const subdomainSlug = getSubdomainSlug(req, domain);
  const customRulesCode = await getCustomWafRules(domain, subdomainSlug);

  logger.waf(`
    [FLOW] Domain: ${domain} | Code Length: ${
      customRulesCode ? customRulesCode.length : 0
    } | URL: ${req.url}`
  );

  const wafReq = {
    method: req.method,
    url: req.url,
    headers: req.headers,
    ip: getClientIp(req),
    body: null,
  };

  try {
    const result = await waf.checkRequestWithCode(
      wafReq,
      customRulesCode,
      domain
    );

    logger.waf(`[FLOW] WAF Result for ${req.url}: ${result.action}`);

    if (result.action === "block") {
      logger.waf(`[ACTION] BLOCKING request for ${req.url} from ${wafReq.ip}`);
      
      let htmlBody = "<h1>Access Denied</h1><p>Request blocked by WAF.</p>";
      try {
        htmlBody = await eta.render("error/waf.ejs", { reason: "blocked" });
        if(!htmlBody) htmlBody = "<h1>Access Denied</h1><p>Request blocked by WAF.</p>";
      } catch (templateErr) {
        console.error("WAF template missing, using fallback:", templateErr.message);
      }

      if (!res.headersSent) {
          res.writeHead(403, { "Content-Type": "text/html" });
          res.end(htmlBody);
      }
      return true;
    }

    if (result.action === "redirect") {
      logger.waf(`[ACTION] REDIRECTING request to ${result.url}`);
      if (!res.headersSent) {
        res.writeHead(302, { Location: result.url });
        res.end();
      }
      return true;
    }

    if (result.action === "challenge") {
      logger.waf(`[ACTION] CHALLENGE issued for ${wafReq.ip}`);
      const token = crypto.randomUUID();
      redis
        .setex(
          `challenge:${token}`,
          300,
          JSON.stringify({
            ip: wafReq.ip,
            ua: req.headers["user-agent"] || "",
            created: Date.now(),
            type: result.type || "basic",
          })
        )
        .catch(console.error);

      let htmlBody = "<h1>Security Check</h1><p>Please verify you are human.</p>";
      try {
        htmlBody = await eta.render("challenge.eta", { token });
        if (!htmlBody) htmlBody = "<h1>Security Check</h1><p>Please verify you are human.</p>";
      } catch (templateErr) {
        console.error("Challenge template missing, using fallback:", templateErr.message);
      }

      if (!res.headersSent) {
        res.writeHead(403, { "Content-Type": "text/html" });
        res.end(htmlBody);
      }
      return true;
    }

    logger.waf(`[FLOW] Allowed request for ${req.url}`);
    return false; 
  } catch (err) {
    logger.error(
      `[WAF ERROR] Uncaught exception during WAF execution for ${domain}:`,
      err.message
    );
    // CRITICAL FIX: If headers were already sent (e.g. partial write), we return true to stop the proxy chain.
    if (res.headersSent) return true;

    // Otherwise, we fail open (allow request) so the site stays up, 
    // unless you want to block on error.
    return false;
  }
}

async function proxyHttp(req, res) {
  try {
    const rawUrl = req.url || "/";
    const host = (req.headers.host || "").split(":")[0];
    if (!host) {
      if (!res.headersSent) res.writeHead(400).end("Missing Host");
      return;
    }
    const method = req.method;

    if (rawUrl.startsWith("/.well-known/acme-challenge/")) {
      const token = rawUrl.split("/").pop();
      const keyAuth = await redis.get(`acme:http:${token}`);
      if (keyAuth) {
        if (!res.headersSent) {
            res.writeHead(200, { "Content-Type": "text/plain" });
            res.end(keyAuth);
        }
        return;
      }
    }

    const urlPath = decodeURIComponent(rawUrl.split("?")[0] || "/");
    const ext = path.extname(urlPath).toLowerCase();
    const isStaticFile =
      urlPath === "/favicon.ico" ||
      urlPath.startsWith("/_next/static/") ||
      urlPath.startsWith("/static/") ||
      [
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
      ].includes(ext);

    if (method === "GET" && isStaticFile) {
      const filePath = path.join(
        process.cwd(),
        "public",
        urlPath.replace(/^\//, "")
      );
      if (fs.existsSync(filePath)) {
        const stats = fs.statSync(filePath);
        if (!res.headersSent) {
            res.writeHead(200, {
            "Content-Length": String(stats.size),
            "Content-Type":
                ext === ".js"
                ? "application/javascript"
                : ext === ".css"
                ? "text/css"
                : "image/x-icon",
            "Cache-Control": urlPath.startsWith("/_next/static/")
                ? "public,max-age=31536000,immutable"
                : "public,max-age=60",
            "x-cache": "LOCAL-FS",
            });
            fs.createReadStream(filePath).pipe(res);
        }
        return;
      }
    }

    let domainData = await getDomainData(host);
    if (!domainData) {
      const { domain: tldtsDomain } = parse(host);
      if (tldtsDomain && tldtsDomain !== host)
        domainData = await getDomainData(tldtsDomain);
    }

    const effectiveDomain = domainData ? domainData.domain : host;
    const wafHandled = await handleWafAndChallenge(req, res, effectiveDomain);
    if (wafHandled) return;

    if (!domainData) {
      if (!res.headersSent) res.writeHead(404).end("Domain not configured");
      return;
    }

    let requestedSlug =
      host === domainData.domain
        ? ""
        : host.endsWith("." + domainData.domain)
        ? host.slice(0, host.length - domainData.domain.length - 1)
        : "";
    const targetService = domainData.proxied?.find(
      (p) =>
        p.slug === requestedSlug || (p.slug === "@" && requestedSlug === "")
    );
    if (!targetService) {
      if (!res.headersSent) res.writeHead(502).end("Unknown host");
      return;
    }

    const cacheKey = `resp:${host}:${rawUrl}`;
    const shouldCache = method === "GET" && targetService.cacheable;
    if (shouldCache) {
      const cached = await getCachedResponse(cacheKey);
      if (cached) {
        if (!res.headersSent) {
            res.writeHead(200, { "Content-Type": "text/html", "x-cache": "HIT" });
            res.end(cached);
        }
        return;
      }
    }

    const ip = getClientIp(req);
    if (targetService.SeperateBannedIP?.some((b) => b.ip === ip)) {
      if (!res.headersSent) res.writeHead(403).end("Forbidden");
      return;
    }

    const protocol = targetService.SSL ? "https" : "http";
    const targetUrl = `${protocol}://${targetService.ip}:${targetService.port}${rawUrl}`;
    const headers = { ...req.headers };
    delete headers.connection;
    delete headers["keep-alive"];
    delete headers["transfer-encoding"];
    delete headers["accept-encoding"];

    const upstream = await request(targetUrl, {
      method,
      headers,
      body: method === "GET" ? undefined : req,
      dispatcher: undiciAgent,
    });
    const outHeaders = {};
    for (const [k, v] of Object.entries(upstream.headers || {})) {
      const lowerK = k.toLowerCase();
      if (
        lowerK !== "content-length" &&
        lowerK !== "transfer-encoding" &&
        lowerK !== "content-encoding"
      )
        outHeaders[k] = v;
    }
    outHeaders["x-powered-by"] = "NetGoat";
    outHeaders["x-tracelet-id"] = crypto.randomUUID();

    if (shouldCache) {
      const bodyBuf = await upstream.body.arrayBuffer();
      const bodyText = Buffer.from(bodyBuf).toString();
      cacheResponse(cacheKey, bodyText, targetService.cacheTTL || 30);
      outHeaders["content-length"] = Buffer.byteLength(bodyText);
      if (!res.headersSent) {
          res.writeHead(upstream.statusCode, outHeaders);
          res.end(bodyText);
      }
    } else {
      if (!res.headersSent) {
          res.writeHead(upstream.statusCode, outHeaders);
          upstream.body.pipe(res);
      }
    }
  } catch (err) {
    console.error("Proxy Error:", err);
    if (!res.headersSent) {
        let html;
        try {
            html = await eta.render("error/500.ejs", {
            error: err.message || String(err),
            });
        } catch(e) {
            html = "<h1>500 Internal Server Error</h1>";
        }
        if (!html) html = "<h1>500 Internal Server Error</h1>";
        
        res.writeHead(500, { "Content-Type": "text/html" });
        res.end(html);
    }
  }
}

async function handleUpgrade(req, socket, head) {
  try {
    const host = (req.headers.host || "").split(":")[0];
    let domainData = await getDomainData(host);
    if (!domainData) {
      const { domain: tldtsDomain } = parse(host);
      if (tldtsDomain && tldtsDomain !== host)
        domainData = await getDomainData(tldtsDomain);
    }
    if (!domainData) {
      socket.destroy();
      return;
    }
    let requestedSlug = host.endsWith("." + domainData.domain)
      ? host.slice(0, host.length - domainData.domain.length - 1)
      : "";
    const targetService = domainData.proxied?.find(
      (p) =>
        p.slug === requestedSlug || (p.slug === "@" && requestedSlug === "")
    );
    if (!targetService || !targetService.WS) {
      socket.destroy();
      return;
    }
    const targetSocket = net.connect(
      targetService.port,
      targetService.ip,
      () => {
        targetSocket.write(head);
        socket.pipe(targetSocket).pipe(socket);
      }
    );
    targetSocket.on("error", () => socket.destroy());
  } catch {
    socket.destroy();
  }
}

async function start() {
  const httpServer = http.createServer(proxyHttp);
  httpServer.timeout = 30000;
  httpServer.on("upgrade", handleUpgrade);
  httpServer.listen(80, () => console.log("Reverse Proxy active (80)"));

  const defaultCert = fs.existsSync(path.join(CERTS_DIR, "default.pem"))
    ? fs.readFileSync(path.join(CERTS_DIR, "default.pem"))
    : null;
  const defaultKey = fs.existsSync(path.join(CERTS_DIR, "default.key"))
    ? fs.readFileSync(path.join(CERTS_DIR, "default.key"))
    : null;

  const options =
    defaultCert && defaultKey
      ? {
          key: defaultKey,
          cert: defaultCert,
          SNICallback: async (servername, cb) => {
            try {
              const domainDoc = await domains
                .findOne({ domain: servername })
                .lean();
              if (!domainDoc) return cb(new Error("No domain"));
              const { cert, key } = await ensureCertificateForUserDomain(
                domainDoc.ownerId,
                servername
              );
              cb(null, tls.createSecureContext({ cert, key }));
            } catch (e) {
              cb(e);
            }
          },
        }
      : {
          key: fs.readFileSync(path.join(CERTS_DIR, "dummy.key")),
          cert: fs.readFileSync(path.join(CERTS_DIR, "dummy.crt")),
        };

  const httpsServer = https.createServer(options, proxyHttp);
  httpsServer.timeout = 30000;
  httpsServer.on("upgrade", handleUpgrade);
  httpsServer.listen(443, () => console.log("Reverse Proxy active (443)"));
}

start();