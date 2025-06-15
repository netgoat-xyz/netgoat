import Fastify from "fastify";
import { request } from "undici";
import { parse } from "tldts";

const fastify = Fastify({ logger: false });

const upstreamMap = {
  "api.example.com": "http://localhost:3000",
  "app.example.com": "http://localhost:3001",
};

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

fastify.all("/*", async (req, reply) => {
  const host = req.headers.host?.split(":")[0];
  const target = upstreamMap[host];
  const { domain, subdomain } = parse(req.hostname);

  if (!target) {
    reply.code(502).send({ error: "Bad Gateway", message: "Unknown host" });
    return;
  }

  try {
    const url = new URL(req.raw.url, target);

    const hasBody = !["GET", "HEAD", "PUT", "POST", "DELETE"].includes(
      req.method
    );

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


    domainLOGS(domain, subdomain, req, new Date(), traceletId);


    return;
  } catch (err) {
    reply.code(500).send({ error: "Proxy error", message: err.message });
  }
});

fastify.listen({ port: 80 }).then(() => {
  logger.info("Reverse proxy running on port 80");
});
