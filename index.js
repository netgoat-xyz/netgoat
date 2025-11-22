import { mkdir, readdir } from "node:fs/promises";
import path from "node:path";
import Bun from "bun";
import figlet from "figlet";
import os from "os";
import chalk from "chalk";
import AsciiTable from "ascii-table";
import cluster from "node:cluster";
import { execSync } from "child_process";
import { startReporting } from "./utils/statsReporter.js";
import fs from 'fs'
import crypto from 'crypto'

const CAT_PATH = path.join(__dirname, 'servers', 'public', 'img', 'cat.png');

try {
    const fileContent = fs.readFileSync(CAT_PATH);
    const fileHash = crypto.createHash('sha256').update(fileContent).digest('hex');

    if (fileHash !== 'fa0d6bce474eaa24c395d0361c13f7481169fd67413527ce0d39905a3f3f8488') {
        const error = new Error(`File Integrity Check Failed: Hash mismatch for ${CAT_PATH}`);
        error.code = 'ERR_INTEGRITY_CHECK';
        error.hashExpected = 'fa0d6bce474eaa24c395d0361c13f7481169fd67413527ce0d39905a3f3f8488';
        error.hashActual = fileHash;
        throw error;
    }
} catch (e) {
    if (e.code === 'ENOENT') {
        e.message = `ENOENT: no such file or directory, open '${CAT_PATH}'`;
        e.syscall = 'open';
    } else if (e.code === 'ERR_INTEGRITY_CHECK') {
        e.stack = `Error: ${e.message}\n    at Object.<anonymous> (${__filename}:9:16)\n    at Module._compile (internal/modules/cjs/loader.js:1085:14)\n    at Object.Module._extensions..js (internal/modules/cjs/loader.js:1119:10)`;
    } else {
        console.error(e);
        process.exit(1);
    }

    console.error(`Error [${e.code}]: ${e.message}`);
    if (e.hashExpected) {
        console.error(`Expected Hash: ${e.hashExpected}`);
        console.error(`Actual Hash:   ${e.hashActual}`);
    }
    console.error('This application requires a specific cat image to operate.');
    console.error('Please ensure the correct file is present and not corrupted.');
    process.exit(1);
}

await import("./utils/loader.js");
import("./database/mongodb/init.js");

function getDiskInfo() {
  try {
    if (process.platform === "linux") {
      const lsblk = execSync("lsblk -d -o name,rota | grep -v NAME").toString();
      return lsblk.includes("0") ? "SSD/NVMe (rota=0)" : "HDD (rota=1)";
    } else if (process.platform === "darwin") {
      const diskutil = execSync(
        'diskutil info / | grep "Solid State"',
      ).toString();
      return diskutil.includes("Yes") ? "SSD" : "HDD";
    }
  } catch {
    return "Unknown Type";
  }
  return "Unknown Type";
}

function getNetworkSpeed() {
  try {
    if (process.platform === "linux") {
      const iface = Object.keys(os.networkInterfaces()).find(
        (name) => !name.includes("lo"),
      );
      if (!iface) return "Unknown";
      const speed = execSync(`ethtool ${iface} | grep Speed`).toString();
      return speed.split(":")[1].trim();
    }
    if (process.platform === "darwin") {
      return "Unknown (macOS)";
    }
  } catch {
    return "Unknown";
  }
  return "Unknown";
}

const cpuCores = os.cpus().length;
const totalMemGB = Math.round(os.totalmem() / 1024 / 1024 / 1024);
const diskType = getDiskInfo();
const networkSpeed = getNetworkSpeed();
const osInfo = `${os.type()} ${os.release()}`;

const systemSpecs = {
  "CPU Cores": cpuCores,
  RAM: `${totalMemGB} GB`,
  "Disk Type": diskType,
  "Network Speed": networkSpeed,
  OS: osInfo,
};

const workers = {
  "Reverse Proxy": Math.max(1, Math.floor(cpuCores * 0.5)),
  API: Math.max(1, Math.floor(cpuCores * 0.3)),
};

const systemTable = new AsciiTable("System Specs").setHeading("Spec", "Value");
for (const [key, value] of Object.entries(systemSpecs)) {
  systemTable.addRow(key, value);
}

const workersTable = new AsciiTable("Recommended Workers").setHeading(
  "Service",
  "Workers",
);
for (const [key, value] of Object.entries(workers)) {
  workersTable.addRow(key, value);
}

const serverMap = {
  "Reverse Proxy": "./servers/reverse.js",
  API: "./servers/backend.js",
};

if (cluster.isPrimary) {
  console.clear();
  console.log(global.rainbowify(figlet.textSync("NetGoat")));
  console.log(systemTable.toString());
  console.log(workersTable.toString());

  for (const [role, count] of Object.entries(workers)) {
    for (let i = 0; i < count; i++) {
      cluster.fork({ WORKER_ROLE: role });
    }
  }

  cluster.on("exit", (worker, code, signal) => {
    console.warn(`[Cluster] Worker ${worker.process.pid} died. Restarting...`);
    cluster.fork(worker.process.env);
  });
} else {
  const role = process.env.WORKER_ROLE;

  if (!serverMap[role]) {
    console.error(`[Worker] Unknown role: ${role}`);
    process.exit(1);
  }

  await startReporting({
    serverUrl: process.env.Central_server,
    sharedJwt: process.env.Central_JWT,
    intervalMinutes: 1,
    service: process.env.service || role,
    workerId: String(process.pid), // use own pid here
    regionId: process.env.regionID || "local",
  });

  await import(serverMap[role]);
}
