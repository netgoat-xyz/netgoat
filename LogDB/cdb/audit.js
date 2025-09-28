const fs = require('fs');
const path = require('path');

const BASE_LOG_DIR = path.join(__dirname, '..', 'logs');
function safeLogPath(filename) {
  const fullPath = path.resolve(BASE_LOG_DIR, filename);
  if (!fullPath.startsWith(BASE_LOG_DIR)) {
    throw new Error('Path traversal detected!');
  }
  return fullPath;
}

if (!fs.existsSync(BASE_LOG_DIR)) fs.mkdirSync(BASE_LOG_DIR, { recursive: true, mode: 0o700 });

class Audit {
  constructor() {
    this.file = safeLogPath('audit.log');
    this.tail = [];
  }

  write(entry) {
    const rec = Object.assign({ ts: new Date().toISOString() }, entry);
    fs.appendFileSync(this.file, JSON.stringify(rec) + '\n', { encoding: 'utf8', mode: 0o600 });
    this.tail.push(rec);
    if (this.tail.length > 200) this.tail.shift();
  }

  last(n = 100) {
    return this.tail.slice(-n);
  }
}

module.exports = Audit;
