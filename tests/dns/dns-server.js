import { serve } from "bun";
import dgram from "dgram";
import dnsPacket from "dns-packet";
import fs from "fs";

const upstreams = ["1.1.1.1", "8.8.8.8"];
let currentUpstream = 0;
function getNextUpstream() {
  const upstream = upstreams[currentUpstream];
  currentUpstream = (currentUpstream + 1) % upstreams.length;
  return upstream;
}

let zones = JSON.parse(fs.readFileSync("./zones.json", "utf8"));
const blockedDomains = new Set(fs.readFileSync("./blocklist.txt", "utf8").split("\\n").filter(Boolean));
const cache = new Map();

function cacheKey(name, type) {
  return name + "|" + type;
}
function getCached(name, type) {
  const key = cacheKey(name, type);
  const entry = cache.get(key);
  if (!entry) return null;
  if (Date.now() > entry.expires) {
    cache.delete(key);
    return null;
  }
  return entry.response;
}
function setCached(name, type, response, ttl = 60) {
  const key = cacheKey(name, type);
  cache.set(key, { response, expires: Date.now() + ttl * 1000 });
}

const udpServer = dgram.createSocket("udp4");

udpServer.on("message", async (msg, rinfo) => {
  const query = dnsPacket.decode(msg);
  const question = query.questions[0];
  const { name, type } = question;

  if (blockedDomains.has(name)) {
    const response = dnsPacket.encode({
      type: "response",
      id: query.id,
      questions: [question],
      answers: [{ name, type: "A", ttl: 60, data: "0.0.0.0" }]
    });
    udpServer.send(response, rinfo.port, rinfo.address);
    return;
  }

  const zoneKey = Object.keys(zones).find(z => name.endsWith(z));
  const zone = zones[zoneKey];
  if (zone && zone[type]) {
    const answers = zone[type].map((rec) => {
      if (type === "MX") return { name, type, ttl: 300, data: rec };
      return { name, type, ttl: 300, data: rec };
    });

    const response = dnsPacket.encode({
      type: "response",
      id: query.id,
      questions: [question],
      answers
    });
    udpServer.send(response, rinfo.port, rinfo.address);
    return;
  }

  const cached = getCached(name, type);
  if (cached) {
    const response = dnsPacket.encode({ ...cached, id: query.id });
    udpServer.send(response, rinfo.port, rinfo.address);
    return;
  }

  const upstream = getNextUpstream();
  const upstreamSocket = dgram.createSocket("udp4");
  upstreamSocket.send(msg, 53, upstream);

  upstreamSocket.on("message", (upstreamMsg) => {
    const decoded = dnsPacket.decode(upstreamMsg);
    setCached(name, type, decoded, decoded.answers[0]?.ttl || 60);
    udpServer.send(upstreamMsg, rinfo.port, rinfo.address);
    upstreamSocket.close();
  });

  upstreamSocket.on("error", () => upstreamSocket.close());
});

udpServer.bind(53, () => console.log("🚀 Ultra-fast DNS Server on :53"));

// Web UI via Bun serve
serve({
  port: 8080,
  async fetch(req) {
    const url = new URL(req.url);
    if (req.method === "GET" && url.pathname === "/zones") {
      return new Response(JSON.stringify(zones), { headers: { "Content-Type": "application/json" } });
    }

    if (req.method === "POST" && url.pathname === "/zones") {
      const body = await req.json();
      zones[body.domain] = body.records;
      fs.writeFileSync("./zones.json", JSON.stringify(zones, null, 2));
      return new Response("OK");
    }

    if (req.method === "DELETE" && url.pathname.startsWith("/zones/")) {
      const domain = decodeURIComponent(url.pathname.split("/").pop());
      delete zones[domain];
      fs.writeFileSync("./zones.json", JSON.stringify(zones, null, 2));
      return new Response("OK");
    }

    return new Response("Not found", { status: 404 });
  }
});