import { Elysia } from "elysia";
import https from "https";
import fs from "fs";
import os from "os";
import cluster from "cluster";
import logger from "./logger";
import { startReporting } from "./statsReporter";
import { cors } from "@elysiajs/cors";
import { Schema, SchemaType, Types, model } from "./cdb/odm";

global.logger = logger;

const numCPUs = os.cpus().length;

if (cluster.isPrimary) {
  logger.success(`Primary process ${process.pid} starting ${numCPUs} workers`);
  for (let i = 0; i < numCPUs; i++) cluster.fork();
  cluster.on("exit", (worker) => {
    logger.warn(`Worker ${worker.process.pid} died. Respawning...`);
    cluster.fork();
  });
} else {
  // HTTPS options: update cert/key paths as needed
  const httpsOptions = {
    key: fs.readFileSync(process.env.SSL_KEY_PATH || "/etc/ssl/private/server.key"),
    cert: fs.readFileSync(process.env.SSL_CERT_PATH || "/etc/ssl/certs/server.crt"),
  };
  const app = new Elysia();
  app.use(cors());

  const Logs = model("Logs", new Schema({
    time: { type: Types.String, default: () => new Date().toISOString() },
    request: { type: Types.String },
    XForwardedFor: { type: Types.String },
    ReqID: { type: Types.String },
    path: { type: Types.String },
    domain: { type: Types.String },
    subdomain: { type: Types.String },
    userAgent: { type: Types.String },
    referer: { type: Types.String },
    remoteAddress: { type: Types.String },
    remotePort: { type: Types.Number },
    requestHeaders: {
      host: { type: Types.String },
      connection: { type: Types.String },
    },
  }));

  // GET analytics
  app.get("/api/:domain/analytics", async ({ params, query, set }) => {
    const domain = params.domain as string;
    const subdomain = typeof query.subdomain === "string" ? query.subdomain : "@";

    try {
      const logs = await Logs.find({ domain, subdomain });
      if (!logs.length) {
        set.status = 404;
        return { error: `No logs found for ${domain}/${subdomain}` };
      }
      return logs;
    } catch (err: any) {
      set.status = 500;
      return { error: `Read error: ${err.message}` };
    }
  });

  // POST analytics
  app.post("/api/:domain/analytics", async ({ params, query, body, set }) => {
    const domain = params.domain as string;
    const subdomain = typeof query.subdomain === "string" ? query.subdomain : "@";

    try {
      const entry = {
        ...body,
        domain,
        subdomain,
        time: new Date().toISOString(),
      };
      await Logs.create(entry);

      set.status = 200;
      return { success: true, message: `Logged to ODM` };
    } catch (err: any) {
      set.status = 500;
      return { error: `Write error: ${err.message}` };
    }
  });

  // Start HTTPS server
  https.createServer(httpsOptions, app.handle).listen(3010, () => {
    logger.success(`[Worker ${process.pid}] LogDB running on https://localhost:3010`);
  });

(async () => {
  await startReporting({
    serverUrl: process.env.Central_server!,
    sharedJwt: process.env.Central_JWT!,
    intervalMinutes: 1,
    service: "LogDB",
    workerId: String(process.pid),
    regionId: process.env.regionID, // ✅ use regionId
    category: "logdb",                      // ✅ choose an allowed category
  });
})();

}
