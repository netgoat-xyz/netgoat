import Fastify from "fastify";
import { registerRoutes } from "./backendstuff/routes.js";
import { registerProxyRoutes } from "./backendstuff/proxyRoutes.js";
import fastifyStatic from "@fastify/static";
import fastifyMultipart from "@fastify/multipart";
import { join } from "path";
import logger from "../utils/logger.js";
import { initializeClickHouse } from "../utils/clickhouseClient.js";

const app = Fastify();

app.register(require('@fastify/cors'), { 
  origin: true,
  credentials: true
})

app.register(fastifyMultipart, {
  limits: {
    fileSize: 1024 * 1024, // 1MB
  }
})

app.register(fastifyStatic, {
  root: join(process.cwd(), "assets"),
  prefix: "/assets/",
  decorateReply: false,
  setHeaders(res, path, stat) {
    res.setHeader("Cache-Control", "public, max-age=31536000, immutable");
  },
  wildcard: false // optional, ensures exact matches
});

app.get("/api/health", async (request, reply) => {
  return { status: "ok", uptime: process.uptime() };
})

// Initialize ClickHouse on startup (non-blocking)
initializeClickHouse().catch((err) => {
  logger.warn("ClickHouse initialization deferred:", err.message);
});

// Register main backend routes
registerRoutes(app);
// Register proxy routes
registerProxyRoutes(app);

const port = process.env.PORT || 3001;

app.listen({ port: 3001, host: '0.0.0.0' }, (err, address) => {
  if (err) {
    console.error(err)
    process.exit(1)
  }
  logger.success("Backend Active on 3001")
})
