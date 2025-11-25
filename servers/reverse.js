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

const CERTS_DIR = path.join(process.cwd(), "database", "certs");
if (!fs.existsSync(CERTS_DIR)) fs.mkdirSync(CERTS_DIR, { recursive: true });

const app = new Elysia();
const eta = new Eta({ views: path.join(process.cwd(), "views") });

const redis = new Redis(process.env.REDIS_URL);
redis.connect().catch(e => console.error("Redis Connection Failed:", e.message));

const WAFRules = new S3Client({
  accessKeyId: process.env.MINIO_ACCESS,
  secretAccessKey: process.env.MINIO_SECRET,
  bucket: "waf-rules",
  endpoint: process.env.MINIO_ENDPOINT, 
});

const SSLCerts = new S3Client({
  accessKeyId: process.env.MINIO_ACCESS,
  secretAccessKey: process.env.MINIO_SECRET,
  bucket: "ssl-certs",
  endpoint: process.env.MINIO_ENDPOINT, 
});

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
  const certKey = `${userId}/${domain}/${subdomain}/fullchain.pem`;
  const privKeyKey = `${userId}/${domain}/${subdomain}/privkey.pem`;

  const s3Cert = SSLCerts.file(certKey);
  const s3Key = SSLCerts.file(privKeyKey);

  if (await s3Cert.exists() && await s3Key.exists()) {
    try {
      const certData = await s3Cert.text();
      const keyData = await s3Key.text();
      const info = acme.openssl.readCertificateInfo(certData);
      if (new Date(info.notAfter).getTime() - Date.now() > 1000 * 60 * 60 * 24 * 14) {
        return { cert: certData, key: keyData };
      }
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

  await SSLCerts.write(certKey, newCert);
  await SSLCerts.write(privKeyKey, privKey);

  return { cert: newCert, key: privKey };
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

async function getCustomWafRules(domain) {
  const cacheKey = `waf:rules:${domain}`;
  
  const cachedRules = await redis.get(cacheKey);
  if (cachedRules) return cachedRules;

  try {
    const s3Key = `custom-rules/${domain}.js`;
    const file = WAFRules.file(s3Key);
    
    if (await file.exists()) {
      const ruleScript = await file.text();
      await redis.setex(cacheKey, 300, ruleScript);
      return ruleScript;
    } else {
      await redis.setex(cacheKey, 300, "");
      return "";
    }
  } catch (err) {
    return null;
  }
}

function cacheResponse(key, value, ttl = 30) {
  redis.setex(key, ttl, value).catch(console.error);
}

async function getCachedResponse(key) { return redis.get(key); }

async function handleWafAndChallenge(req, res, domain) {
  const customRulesCode = await getCustomWafRules(domain);

  const wafReq = {
    method: req.method,
    url: req.url,
    headers: req.headers,
    ip: getClientIp(req),
    body: null 
  };

  const result = await waf.checkRequest(wafReq, customRulesCode);

  if (result.action === "block") { 
    res.writeHead(403, { "Content-Type": "text/html" }); 
    res.end(await eta.render("error/waf.ejs", { reason: "blocked" })); 
    return true; 
  }
  
  if (result.action === "redirect") { 
    res.writeHead(302, { Location: result.url }); 
    res.end(); 
    return true; 
  }

  if (result.action === "challenge") {
    const token = crypto.randomUUID();
    redis.setex(`challenge:${token}`, 300, JSON.stringify({ ip: wafReq.ip, ua: req.headers["user-agent"] || "", created: Date.now(), type: result.type || "basic" })).catch(console.error);
    const html = await eta.render("challenge.eta", { token });
    res.writeHead(403, { "Content-Type": "text/html" }); res.end(html);
    return true;
  }
  return false;
}

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

    let domainData = await getDomainData(host);
    if (!domainData) {
      const { domain: tldtsDomain } = parse(host);
      if (tldtsDomain && tldtsDomain !== host) domainData = await getDomainData(tldtsDomain);
    }
    
    const effectiveDomain = domainData ? domainData.domain : host;
    const wafHandled = await handleWafAndChallenge(req, res, effectiveDomain);
    if (wafHandled) return;

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

async function start() {
  const httpServer = http.createServer(proxyHttp);
  httpServer.timeout = 30000;
  httpServer.on("upgrade", handleUpgrade);
  httpServer.listen(80, ()=> console.log("Reverse Proxy active (80)"));

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
  httpsServer.listen(443, ()=> console.log("Reverse Proxy active (443)"));
}

start();