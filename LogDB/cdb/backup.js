const fs = require('fs');
const path = require('path');

const DATA_DIR = path.join(__dirname, '..', 'data');
const BACKUP_DIR = path.join(__dirname, '..', 'backups');
if (!fs.existsSync(BACKUP_DIR)) fs.mkdirSync(BACKUP_DIR, { recursive: true });

function createBackup(name) {
  const ts = Date.now();
  const dest = path.join(BACKUP_DIR, `${name || 'backup'}-${ts}.zip`);
  // For demo: just copy files into a folder (no real zip to avoid deps)
  const folder = dest.replace('.zip', '');
  fs.mkdirSync(folder, { recursive: true });
  if (fs.existsSync(DATA_DIR)) {
    for (const f of fs.readdirSync(DATA_DIR)) {
      fs.copyFileSync(path.join(DATA_DIR, f), path.join(folder, f));
    }
  }
  return folder;
}

module.exports = { createBackup };
