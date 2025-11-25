import fs from "fs/promises";
import path from "path";
import fsSync from "fs";

// Cache Directory configuration
const TEMP_DIR = path.join(process.cwd(), "database", "s3_cache");
if (!fsSync.existsSync(TEMP_DIR)) fsSync.mkdirSync(TEMP_DIR, { recursive: true });

export default class S3Filesystem {
  constructor(s3Client, cacheDirName) {
    this.client = s3Client;
    this.cacheDir = path.join(TEMP_DIR, cacheDirName);
    if (!fsSync.existsSync(this.cacheDir)) fsSync.mkdirSync(this.cacheDir, { recursive: true });
    this.TTL_MS = 30 * 60 * 1000; // 30 minutes
  }

  // Converts S3 key (e.g., user/domain/key.pem) to local safe path
  _getLocalPath(s3Key) {
    // Replace characters that might be unsafe for filenames, but keep slashes for structure
    const safeKey = s3Key.replace(/[^a-zA-Z0-9.\-_/]/g, '_');
    return path.join(this.cacheDir, safeKey);
  }

  // Helper to ensure the directory for a specific file exists
  async _ensureDir(filePath) {
    const dir = path.dirname(filePath);
    try {
      await fs.access(dir);
    } catch {
      await fs.mkdir(dir, { recursive: true });
    }
  }

  // --- Read Operations ---
  async read(s3Key) {
    const localPath = this._getLocalPath(s3Key);

    try {
      // 1. Check local cache
      const stats = await fs.stat(localPath);
      if (Date.now() - stats.mtimeMs < this.TTL_MS) {
        return { 
          content: await fs.readFile(localPath), 
          source: 'cache',
          path: localPath
        };
      }
    } catch (e) {
      // File not found in cache or expired
    }

    // 2. Fetch from S3
    try {
      const file = this.client.file(s3Key);
      if (!(await file.exists())) {
        return null;
      }
      const content = await file.arrayBuffer();
      const contentBuffer = Buffer.from(content);

      // 3. Store locally before returning (FIXED: Ensure dir exists first)
      await this._ensureDir(localPath);
      await fs.writeFile(localPath, contentBuffer);
      
      return { 
        content: contentBuffer, 
        source: 's3',
        path: localPath
      };

    } catch (e) {
      console.error(`S3 Read Error for key ${s3Key}:`, e.message);
      return null;
    }
  }

  // --- Write Operations ---
  async write(s3Key, data) {
    // 1. Write to S3
    await this.client.write(s3Key, data);

    // 2. Write to local cache (FIXED: Ensure dir exists first)
    const localPath = this._getLocalPath(s3Key);
    await this._ensureDir(localPath);
    await fs.writeFile(localPath, data);
  }

  // --- Utility Operations ---
  async exists(s3Key) {
    const localPath = this._getLocalPath(s3Key);
    try {
      const stats = await fs.stat(localPath);
      if (Date.now() - stats.mtimeMs < this.TTL_MS) return true;
    } catch (e) {
      // Not in cache, check S3
    }
    
    return this.client.file(s3Key).exists();
  }
}