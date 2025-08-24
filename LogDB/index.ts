import { Elysia } from 'elysia';
import path from 'path';
import os from 'os';
import cluster from 'cluster';
import logger from './logger';
import { startReporting } from './statsReporter';
import { cors } from '@elysiajs/cors'
import fs from "fs/promises"

global.logger = logger;

const BASE_LOG_DIR = path.resolve(process.cwd(), 'database/DomainLogs');
const numCPUs = os.cpus().length;

if (cluster.isPrimary) {
  logger.success(`Primary process ${process.pid} starting ${numCPUs} workers`);
  for (let i = 0; i < numCPUs; i++) cluster.fork();
  cluster.on('exit', (worker) => {
    logger.warn(`Worker ${worker.process.pid} died. Respawning...`);
    cluster.fork();
  });
} else {
  const app = new Elysia();
  app.use(cors())

  app.get('/api/:domain/analytics', async ({ params, query, set }) => {
    const domain = params.domain;
    const subdomain = typeof query.subdomain === 'string' ? query.subdomain : '@';
    const timeframe = typeof query.timeframe === 'string' ? query.timeframe : '7d';
    const logFilePath = path.join(BASE_LOG_DIR, domain, subdomain, `${timeframe}.json`);

    try {
      const file = Bun.file(logFilePath);
      if (!(await file.exists())) {
        set.status = 404;
        return { error: `No log found for ${domain}/${subdomain}/${timeframe}` };
      }
      const text = await file.text();
      return JSON.parse(text);
    } catch (err) {
      set.status = 500;
      return { error: `Read error: ${err.message}` };
    }
  });


// ...

app.post('/api/:domain/analytics', async ({ params, query, body, set }) => {
  const domain = params.domain
  const subdomain = typeof query.subdomain === 'string' ? query.subdomain : '@'
  const timeframe = typeof query.timeframe === 'string' ? query.timeframe : '1d'
  const logFilePath = path.join(BASE_LOG_DIR, domain, subdomain, `${timeframe}.json`)
  const logDir = path.dirname(logFilePath)

  try {
    await fs.mkdir(logDir, { recursive: true })   // <-- use fs.mkdir

    let logs = []
    const file = Bun.file(logFilePath)
    if (await file.exists()) {
      try {
        const text = await file.text()
        logs = JSON.parse(text)
      } catch (err) {
        logger.warn(`Failed to parse existing log at ${logFilePath}:`, err)
      }
    }

    const entry = typeof body === 'object' && body !== null ? { ...body } : {}
    entry.time ??= new Date().toISOString()

    logs.push(entry)
    await Bun.write(logFilePath, JSON.stringify(logs, null, 2))

    set.status = 200
    return { success: true, message: `Logged to ${logFilePath}` }
  } catch (err) {
    set.status = 500
    return { error: `Write error: ${err.message}` }
  }
})

  app.listen({ port: 3010 });
  logger.success(`[Worker ${process.pid}] LogDB running on http://localhost:3010`);
  logger.debug(`[Worker ${process.pid}] Logs stored in: ${BASE_LOG_DIR}`);

  (async () => {
    await startReporting({
      serverUrl: process.env.Central_server,
      sharedJwt: process.env.Central_JWT,
      intervalMinutes: 1,
      service: 'LogDB',
      workerId: String(process.pid),
      regionId: process.env.regionID || 'local'
    });
  })();
}
