import Fastify from "fastify";
import mongoose from "mongoose";
import { registerRoutes } from "./backendstuff/routes.js";
import { registerProxyRoutes } from "./backendstuff/proxyRoutes.js";
import fastifyStatic from "@fastify/static";
import { join } from "path";
const app = Fastify();

app.register(require('@fastify/cors'), { 
  origin: true,
  credentials: true
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
  console.log(`Backend loaded at ${address}`)
})
