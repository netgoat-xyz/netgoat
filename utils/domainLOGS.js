import { mkdir, stat, readFile } from "node:fs/promises";
import Bun from "bun";
import path from "node:path";

async function domainLOGS(
  domain,
  subdomain,
  req,
  time = new Date(),
  traceletId
) {
  const schema = {
    time: time.toISOString(),
    request: req.method,
    XForwardedFor: req.headers["x-forwarded-for"] || req.ip,
    ReqID: traceletId,
    path: req.url,
    domain: domain,
    subdomain: subdomain,
    userAgent: req.headers["user-agent"] || "Unknown",
    referer: req.headers["referer"] || "Unknown",
    statusCode: req.statusCode,
    remoteAddress: req.raw.socket.remoteAddress || "Unknown",
    remotePort: req.raw.socket.remotePort || "Unknown",
    requestHeaders: req.headers,
  };

  const baseDir = path.join(process.cwd(), "database", "DomainLogs");
  const domainDir = path.join(baseDir, domain);
  const subdomainDir = path.join(domainDir, subdomain || "_");

  async function ensureDirExists(dir) {
    try {
      await stat(dir);
    } catch {
      await mkdir(dir, { recursive: true });
    }
  }

  await ensureDirExists(subdomainDir);

  const logFileNames = ["30d.json", "7d.json", "1d.json"];
  const writePromises = logFileNames.map(async (file) => {
    const filePath = path.join(subdomainDir, file);
    let logs = [];
    try {
      const existing = await readFile(filePath, "utf-8");
      logs = JSON.parse(existing);
      if (!Array.isArray(logs)) logs = [];
    } catch {
      logs = [];
    }
    logs.push(schema);
    return Bun.write(filePath, JSON.stringify(logs, null, 2));
  });

  await Promise.all(writePromises);
}

export default domainLOGS;
