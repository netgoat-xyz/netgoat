import Fastify from "fastify";
import mongoose from "mongoose";
import { registerRoutes } from "./backendstuff/routes.js";
import { registerProxyRoutes } from "./backendstuff/proxyRoutes.js";
const app = Fastify();
app.register(require('@fastify/cors'), { 
  // put your options here
})

// Register main backend routes
registerRoutes(app);
// Register proxy routes
registerProxyRoutes(app);

app.listen({ port: 3001 }, (err, address) => {
  if (err) {
    console.error(err);
    process.exit(1);
  }
  console.log(`Backend loaded at ${address}`);
});
