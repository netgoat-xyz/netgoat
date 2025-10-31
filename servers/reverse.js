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
import mongoose from "mongoose";
import acme from "acme-client";
import WAF from "../utils/ruleScript.js";
import Users from "../database/mongodb/schema/users.js";
import domains from "../database/mongodb/schema/domains.js";
import Score from "../database/mongodb/schema/score.js";

// --- Config & Setup ---
const CERTS_DIR = path.join(process.cwd(), "database", "certs");
if (!fs.existsSync(CERTS_DIR)) fs.mkdirSync(CERTS_DIR, { recursive: true });

const app = new Elysia();
const eta = new Eta({ views: path.join(process.cwd(), "views") });
const redis = new Redis(process.env.REDIS_URL);
redis.connect().catch(e => console.error("Redis Connection Failed:", e.message));

const waf = new WAF();
const undiciAgent = new Agent({ connections: 100, pipelining: 1 });
http.globalAgent.keepAlive = true;
https.globalAgent.keepAlive = true;

const domainMemoryCache = new Map(); // in-memory cache for hot domains

// --- Helpers ---
function getClientIp(req) {
  const xff = req.headers["x-forwarded-for"];
  if (xff) return xff.split(",")[0].trim();
  return req.headers["x-real-ip"] || req.socket.remoteAddress || "unknown";
}

function getCertPaths(userId, domain, subdomain = "@") {
  const base = path.join(CERTS_DIR, userId, domain, subdomain);
  return {
    dir: base,
    cert: path.join(base, "fullchain.pem"),
    key: path.join(base, "privkey.pem")
  };
}

async function loadOrCreateAccountKey() {
  const ACCOUNT_KEY_PATH = path.join(CERTS_DIR, "acme-account.key");
  if (fs.existsSync(ACCOUNT_KEY_PATH)) return fs.readFileSync(ACCOUNT_KEY_PATH, "utf8");
  const key = await acme.openssl.createPrivateKey();
  fs.writeFileSync(ACCOUNT_KEY_PATH, key, { mode: 0o600 });
  return key;
}

async function getAcmeClient() {
  const accountKey = await loadOrCreateAccountKey();
  return new acme.Client({
    directoryUrl: acme.directory.letsencrypt.production,
    accountKey
  });
}

async function ensureCertificateForUserDomain(userId, domain, subdomain = "@") {
  const { dir, cert, key } = getCertPaths(userId, domain, subdomain);
  if (fs.existsSync(cert) && fs.existsSync(key)) {
    try {
      const certData = fs.readFileSync(cert, "utf8");
      const info = acme.openssl.readCertificateInfo(certData);
      if (new Date(info.notAfter).getTime() - Date.now() > 1000 * 60 * 60 * 24 * 14)
        return { cert: fs.readFileSync(cert), key: fs.readFileSync(key) };
    } catch {}
  }
  const client = await getAcmeClient();
  const [privKey, csr] = await acme.openssl.createCSR({ commonName: domain });
  const order = await client.createOrder({ identifiers: [{ type: "dns", value: domain }] });
  const authzs = await client.getAuthorizations(order);
  for (const auth of authzs) {
    const challenge = auth.challenges.find(c => c.type === "http-01");
    const keyAuth = await client.getChallengeKeyAuthorization(challenge);
    await redis.setex(`acme:http:${challenge.token}`, 300, keyAuth);
    await client.verifyChallenge(auth, challenge);
    await client.completeChallenge(challenge);
    await client.waitForValidStatus(challenge);
  }
  const finalized = await client.finalizeOrder(order, csr);
  const newCert = await client.getCertificate(finalized);
  if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
  fs.writeFileSync(cert, newCert);
  fs.writeFileSync(key, privKey, { mode: 0o600 });
  return { cert: fs.readFileSync(cert), key: fs.readFileSync(key) };
}

// --- Domain caching ---
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

function cacheResponse(key, value, ttl = 30) {
  redis.setex(key, ttl, value).catch(console.error);
}

async function getCachedResponse(key) { return redis.get(key); }

// --- WAF + challenge ---
async function handleWafAndChallenge(req, res) {
  const wafReq = {
    method: req.method,
    url: req.url,
    headers: req.headers,
    ip: getClientIp(req),
    body: null
  };
  const result = await waf.checkRequest(wafReq);
  if (result.action === "block") { res.writeHead(403, { "Content-Type": "text/html" }); res.end(await eta.render("error/waf.ejs", { reason: "blocked" })); return true; }
  if (result.action === "redirect") { res.writeHead(302, { Location: result.url }); res.end(); return true; }
  if (result.action === "challenge") {
    const token = crypto.randomUUID();
    redis.setex(`challenge:${token}`, 300, JSON.stringify({ ip: wafReq.ip, ua: req.headers["user-agent"] || "", created: Date.now(), type: result.type || "basic" })).catch(console.error);
    const html = await eta.render("challenge.eta", { token });
    res.writeHead(403, { "Content-Type": "text/html" }); res.end(html);
    return true;
  }
  return false;
}

// --- HTTP Proxy ---
async function proxyHttp(req, res) {
  try {
    const rawUrl = req.url || "/";
    const host = (req.headers.host || "").split(":")[0];
    if (!host) { res.writeHead(400).end("Missing Host"); return; }
    const method = req.method;

    if (rawUrl.startsWith("/.well-known/acme-challenge/")) {
      const token = rawUrl.split("/").pop();
      const keyAuth = await redis.get(`acme:http:${token}`);
      if (keyAuth) { res.writeHead(200, { "Content-Type": "text/plain" }); res.end(keyAuth); return; }
    }

    const urlPath = decodeURIComponent(rawUrl.split("?")[0] || "/");
    const ext = path.extname(urlPath).toLowerCase();
    const isStaticFile = urlPath === "/favicon.ico" || urlPath.startsWith("/_next/static/") || urlPath.startsWith("/static/") || [".js",".css",".ico",".png",".jpg",".jpeg",".svg",".webp",".json",".wasm",".map"].includes(ext);

    if (method === "GET" && isStaticFile) {
      const filePath = path.join(process.cwd(), "public", urlPath.replace(/^\//, ""));
      if (fs.existsSync(filePath)) {
        const stats = fs.statSync(filePath);
        res.writeHead(200, {
          "Content-Length": String(stats.size),
          "Content-Type": ext === ".js" ? "application/javascript" : ext === ".css" ? "text/css" : "image/x-icon",
          "Cache-Control": urlPath.startsWith("/_next/static/") ? "public,max-age=31536000,immutable" : "public,max-age=60",
          "x-cache": "LOCAL-FS"
        });
        fs.createReadStream(filePath).pipe(res);
        return;
      }
    }

    const wafHandled = await handleWafAndChallenge(req, res);
    if (wafHandled) return;

    let domainData = await getDomainData(host);
    if (!domainData) {
      const { domain: tldtsDomain } = parse(host);
      if (tldtsDomain && tldtsDomain !== host) domainData = await getDomainData(tldtsDomain);
    }
    if (!domainData) { res.writeHead(404).end("Domain not configured"); return; }

    let requestedSlug = host === domainData.domain ? "" : host.endsWith('.'+domainData.domain) ? host.slice(0, host.length-domainData.domain.length-1) : "";
    const targetService = domainData.proxied?.find(p => p.slug === requestedSlug || (p.slug === "@" && requestedSlug === ""));
    if (!targetService) { res.writeHead(502).end("Unknown host"); return; }

    const cacheKey = `resp:${host}:${rawUrl}`;
    const shouldCache = method === "GET" && targetService.cacheable;
    if (shouldCache) {
      const cached = await getCachedResponse(cacheKey);
      if (cached) { res.writeHead(200, { "Content-Type": "text/html", "x-cache":"HIT" }); res.end(cached); return; }
    }

    const ip = getClientIp(req);
    if (targetService.SeperateBannedIP?.some(b => b.ip === ip)) { res.writeHead(403).end("Forbidden"); return; }

    const protocol = targetService.SSL ? "https" : "http";
    const targetUrl = `${protocol}://${targetService.ip}:${targetService.port}${rawUrl}`;
    const headers = { ...req.headers };
    delete headers.connection; delete headers["keep-alive"]; delete headers["transfer-encoding"]; delete headers["accept-encoding"];
    
    const upstream = await request(targetUrl, { method, headers, body: method === "GET" ? undefined : req, dispatcher: undiciAgent });
    const outHeaders = {};
    for (const [k,v] of Object.entries(upstream.headers||{})) {
      const lowerK = k.toLowerCase();
      if (lowerK !== "content-length" && lowerK !== "transfer-encoding" && lowerK !== "content-encoding") outHeaders[k] = v;
    }
    outHeaders["x-powered-by"] = "NetGoat";
    outHeaders["x-tracelet-id"] = crypto.randomUUID();

    if (shouldCache) {
      const bodyBuf = await upstream.body.arrayBuffer();
      const bodyText = Buffer.from(bodyBuf).toString();
      cacheResponse(cacheKey, bodyText, targetService.cacheTTL||30);
      outHeaders["content-length"] = Buffer.byteLength(bodyText);
      res.writeHead(upstream.statusCode, outHeaders);
      res.end(bodyText);
    } else {
      res.writeHead(upstream.statusCode, outHeaders);
      upstream.body.pipe(res);
    }
  } catch(err) {
    console.error("Proxy Error:", err);
    const html = await eta.render("error/500.ejs", { error: err.message||String(err) });
    res.writeHead(500, { "Content-Type": "text/html" });
    res.end(html);
  }
}

// --- WS / TCP upgrade ---
async function handleUpgrade(req, socket, head) {
  try {
    const host = (req.headers.host||"").split(":")[0];
    let domainData = await getDomainData(host);
    if (!domainData) {
      const { domain: tldtsDomain } = parse(host);
      if (tldtsDomain && tldtsDomain !== host) domainData = await getDomainData(tldtsDomain);
    }
    if (!domainData) { socket.destroy(); return; }
    let requestedSlug = host.endsWith('.'+domainData.domain) ? host.slice(0, host.length-domainData.domain.length-1) : "";
    const targetService = domainData.proxied?.find(p => p.slug === requestedSlug || (p.slug==="@" && requestedSlug===""));
    if (!targetService || !targetService.WS) { socket.destroy(); return; }
    const targetSocket = net.connect(targetService.port, targetService.ip, () => {
      targetSocket.write(head); socket.pipe(targetSocket).pipe(socket);
    });
    targetSocket.on("error", ()=>socket.destroy());
  } catch { socket.destroy(); }
}

// --- Start Servers ---
async function start() {
  const httpServer = http.createServer(proxyHttp);
  httpServer.timeout = 30000;
  httpServer.on("upgrade", handleUpgrade);
  httpServer.listen(80, ()=>console.log("HTTP listening on 80"));

  const defaultCert = fs.existsSync(path.join(CERTS_DIR,"default.pem")) ? fs.readFileSync(path.join(CERTS_DIR,"default.pem")) : null;
  const defaultKey = fs.existsSync(path.join(CERTS_DIR,"default.key")) ? fs.readFileSync(path.join(CERTS_DIR,"default.key")) : null;

  const options = defaultCert && defaultKey ? {
    key: defaultKey,
    cert: defaultCert,
    SNICallback: async (servername, cb) => {
      try {
        const domainDoc = await domains.findOne({ domain: servername }).lean();
        if (!domainDoc) return cb(new Error("No domain"));
        const { cert, key } = await ensureCertificateForUserDomain(domainDoc.ownerId, servername);
        cb(null, tls.createSecureContext({ cert, key }));
      } catch(e){ cb(e); }
    }
  } : {
    key: fs.readFileSync(path.join(CERTS_DIR,"dummy.key")),
    cert: fs.readFileSync(path.join(CERTS_DIR,"dummy.crt"))
  };

  const httpsServer = https.createServer(options, proxyHttp);
  httpsServer.timeout = 30000;
  httpsServer.on("upgrade", handleUpgrade);
  httpsServer.listen(443, ()=>console.log("HTTPS listening on 443"));
}

start();
