const fs = require('fs');
const path = require('path');

const LOG_DIR = path.join(__dirname, '..', 'logs');
if (!fs.existsSync(LOG_DIR)) fs.mkdirSync(LOG_DIR, { recursive: true });

class Audit {
  constructor() {
    this.file = path.join(LOG_DIR, 'audit.log');
    this.tail = [];
  }

  write(entry) {
    const rec = Object.assign({ ts: new Date().toISOString() }, entry);
    fs.appendFileSync(this.file, JSON.stringify(rec) + '\n', 'utf8');
    this.tail.push(rec);
    if (this.tail.length > 200) this.tail.shift();
  }

  last(n = 100) {
    return this.tail.slice(-n);
  }
}

module.exports = Audit;
