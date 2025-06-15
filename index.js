
import { mkdir, readdir } from "node:fs/promises";
import Bun from "bun"
import path from "node:path";
import figlet from 'figlet';
import os from 'os';
import chalk from 'chalk';
import AsciiTable from 'ascii-table'; 
import { initRaft } from "./electionRaft/index.js";

await import('./utils/loader.js');

const text = figlet.textSync('NetGoat');
console.log(rainbowify(text));
import { execSync } from 'child_process';

function getDiskInfo() {
  try {
    // Linux/macOS: get disk type (NVMe/SATA/SSD) from lsblk or diskutil
    if (process.platform === 'linux') {
      const lsblk = execSync('lsblk -d -o name,rota | grep -v NAME').toString();
      // rota=0 means SSD, 1 means HDD
      if (lsblk.includes('0')) return 'SSD/NVMe (rota=0)';
      else return 'HDD (rota=1)';
    } else if (process.platform === 'darwin') {
      const diskutil = execSync('diskutil info / | grep "Solid State"').toString();
      return diskutil.includes('Yes') ? 'SSD' : 'HDD';
    }
  } catch {
    return 'Unknown';
  }
  return 'Unknown';
}

function getNetworkSpeed() {
  try {
    if (process.platform === 'linux') {
      const iface = Object.keys(os.networkInterfaces()).find(name => !name.includes('lo'));
      if (!iface) return 'Unknown';
      const speed = execSync(`ethtool ${iface} | grep Speed`).toString();
      return speed.split(':')[1].trim();
    }
    if (process.platform === 'darwin') {
      // macOS doesn't have ethtool by default, fallback to generic
      return 'Unknown (macOS)';
    }
  } catch {
    return 'Unknown';
  }
  return 'Unknown';
}

const cpuCores = os.cpus().length;
const totalMemGB = Math.round(os.totalmem() / 1024 / 1024 / 1024);
const diskType = getDiskInfo();
const networkSpeed = getNetworkSpeed();
const osInfo = `${os.type()} ${os.release()}`;

const systemSpecs = {
  'CPU Cores': cpuCores,
  'RAM': `${totalMemGB} GB`,
  'Disk Type': diskType,
  'Network Speed': networkSpeed,
  'OS': osInfo,
};

const workers = {
  'Reverse Proxy': Math.max(1, Math.floor(cpuCores * 0.5)),
  'API': Math.max(1, Math.floor(cpuCores * 0.3)),
  'Frontend': Math.max(1, Math.floor(cpuCores * 0.2)),
};

const systemTable = new AsciiTable('System Specs');
systemTable.setHeading('Spec', 'Value');

for (const [key, value] of Object.entries(systemSpecs)) {
  systemTable.addRow(key, value);
}

const workersTable = new AsciiTable('Recommended Workers');
workersTable.setHeading('Service', 'Workers');

for (const [key, value] of Object.entries(workers)) {
  workersTable.addRow(key, value);
}

console.clear();
console.log(rainbowify(figlet.textSync('NetGoat')));
console.log(systemTable.toString());
console.log(workersTable.toString());


let NODE_ID = process.env.NODE_ID;
let PORT = process.env.ElectionPort;
let PEERS = process.env.PEERS ? process.env.PEERS.split(",") : [];
let shardManagerUrl = process.env.SHARD_MANAGER_URL;

// initRaft(NODE_ID, PORT, PEERS, shardManagerUrl);

await Promise.all([
  import('./servers/backend.js'),
  import('./servers/frontend.js'),
  import('./servers/reverse.js'),
]);
