import Fastify from "fastify";
import { request } from "undici";
import { parse } from "tldts";
import domains from "../database/mongodb/schema/domains";
import Score from "../database/mongodb/schema/score.js";

const fastify = Fastify({ logger: false });

function getClientIp(req) {
  const xForwardedFor = req.headers["x-forwarded-for"];
  if (xForwardedFor) return xForwardedFor.split(",")[0].trim();
  return req.ip || req.raw.connection.remoteAddress || "unknown";
}

fastify.addHook("onRequest", (req, reply, done) => {
  logger.info(
    `incoming request: ${req.method} ${req.url} Host: ${req.headers.host}`
  );
  done();
});

fastify.addHook("onResponse", (req, reply, done) => {
  logger.info(
    `request completed: ${req.method} ${req.url} Status: ${
      reply.statusCode
    } in ${reply.getResponseTime().toFixed(2)}ms`
  );
  done();
});

// Cachey Cachey
const domainCache = new Map();

fastify.all("/*", async (req, reply) => {
  try {
    const host = req.headers.host?.split(":")[0];
    if (!host)
      return reply
        .code(400)
        .send({ error: "Bad Request", message: "Missing Host header" });

    const { domain, subdomain } = parse(host);
    if (!domain)
      return reply
        .code(400)
        .send({ error: "Bad Request", message: "Invalid domain" });

    let domainData = domainCache.get(domain);
    if (!domainData) {
      domainData = await domains.findOne({ domain });
      if (domainData) domainCache.set(domain, domainData);
    }

    const target = domainData?.proxied || null;
    if (!target)
      return reply
        .code(502)
        .send({ error: "Bad Gateway", message: "Unknown host" });

    const url = new URL(req.raw.url, target);

    const ipAddress = getClientIp(req);

    const agg = await Score.aggregate([
      { $match: { ipAddress } },
      {
        $group: {
          _id: "$ipAddress",
          totalScore: { $sum: "$score" },
          count: { $sum: 1 },
        },
      },
    ]);

    if (agg.length > 60) {
      logger.warn(
        `IP ${ipAddress} has exceeded the score with ${agg.length} requests.`
      );
    }

    // Body Check, hooooray.
    const methodsWithBody = ["POST", "PUT", "PATCH", "DELETE"];
    const hasBody = methodsWithBody.includes(req.method.toUpperCase());

    const upstreamReq = await request(url.toString(), {
      method: req.method,
      headers: req.headers,
      body: hasBody ? req.raw : undefined,
    });

    const traceletId = tracelet(process.env.regionID);

    reply.status(upstreamReq.statusCode);
    reply.header(
      "Access-Control-Expose-Headers",
      "x-tracelet-id, x-powered-by, x-worker-id"
    );
    reply.header("x-tracelet-id", traceletId);
    reply.header("x-powered-by", "NetGoat Reverse Proxy");
    
    for (const [k, v] of Object.entries(upstreamReq.headers)) {
      if (k.toLowerCase() === "transfer-encoding") continue;
      reply.header(k, v);
    }

    // Record and waste space for user's sake
    domainLOGS(domain, subdomain, req, new Date(), traceletId);

    // Score Tracker (Im gonna start tweaking)
    try {
      const scoreValue = 1; // Tweak this you lil bastard

      const newScore = new Score({ ipAddress, score: scoreValue });
      await newScore.save();
    } catch (err) {
      logger.error("Failed to save request score:", err);
    }
    // ==============================

    // Pipe upstream response back
    const responseBody = await upstreamReq.body.text();
const contentType = upstreamReq.headers["content-type"] || "";

if (contentType.includes("text/html")) {
  const injectedScript = `
    <script src="https://unpkg.com/rrweb@latest/dist/rrweb.min.js"></script>
    <script src="https://api.netgoat.cloudable.dev/monitor.js"></script>
  `;

  const modifiedBody = responseBody.replace('</body>', `${injectedScript}</body>`);
  return reply.send(modifiedBody);
} else {
  return reply.send(responseBody);
}
  } catch (err) {
    return reply.code(500).send({ error: "Proxy error", message: err.message });
  }
});

fastify.listen({ port: 80 }).then(() => {
  logger.info("Reverse proxy running on port 80");
});
