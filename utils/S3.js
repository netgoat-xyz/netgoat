import fs from "fs/promises";
import path from "path";
import fsSync from "fs";

// Cache Directory configuration
// Use path.resolve to get an absolute, canonical path for robust path traversal checks
const TEMP_DIR = path.resolve(process.cwd(), "database", "s3_cache");
if (!fsSync.existsSync(TEMP_DIR)) fsSync.mkdirSync(TEMP_DIR, { recursive: true });

export default class S3Filesystem {
  constructor(s3Client, cacheDirName) {
    this.client = s3Client;
    // Resolve cacheDir to ensure it's an absolute path for security checks
    this.cacheDir = path.resolve(TEMP_DIR, cacheDirName);
    if (!fsSync.existsSync(this.cacheDir)) fsSync.mkdirSync(this.cacheDir, { recursive: true });
    this.TTL_MS = 30 * 60 * 1000; // 30 minutes
  }

  /**
   * Converts S3 key (e.g., user/domain/key.pem) to a safe local path within the cache directory.
   * Includes a security check to prevent path traversal attacks.
   * @param {string} s3Key The key from S3, potentially user-controlled.
   * @returns {string} The safe, absolute local file path.
   * @throws {Error} If a path traversal attempt is detected.
   */
  _getLocalPath(s3Key) {
    // Replace characters that might be unsafe for filenames, but keep slashes for structure.
    const safeKey = s3Key.replace(/[^a-zA-Z0-9.\-_/]/g, '_');
    
    // Construct the full path using the key. path.join handles OS-specific separators.
    const fullPath = path.join(this.cacheDir, safeKey);
    
    // Normalize the path (resolves '..', '.' sequences).
    const normalizedPath = path.normalize(fullPath);

    // SECURITY CHECK: Use path.relative to verify the path is contained within the cache directory.
    // If the relative path starts with '..', it means a traversal attempt was made to move 
    // outside the base directory.
    const relativePath = path.relative(this.cacheDir, normalizedPath);
    if (relativePath.startsWith('..') || path.isAbsolute(relativePath)) {
      console.warn(`Path traversal attempt blocked for key: ${s3Key}`);
      throw new Error(`Invalid path derivation: Path traversal attempt detected for key: ${s3Key}`);
    }

    return normalizedPath;
  }

  // Helper to ensure the directory for a specific file exists
  async _ensureDir(filePath) {
    const dir = path.dirname(filePath);
    try {
      // Use fs.access to check existence quickly without reading stats
      await fs.access(dir);
    } catch {
      // If access fails (doesn't exist), create it
      await fs.mkdir(dir, { recursive: true });
    }
  }

  // --- Read Operations ---
  async read(s3Key) {
    let localPath;
    try {
      // Path traversal check is performed here
      localPath = this._getLocalPath(s3Key);
    } catch (e) {
      // Handle path traversal error from _getLocalPath
      console.error(`Rejected access attempt for key ${s3Key}:`, e.message);
      return null;
    }
    
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
      // File not found in cache or expired. Continue to S3 fetch.
    }

    // 2. Fetch from S3
    try {
      const file = this.client.file(s3Key);
      if (!(await file.exists())) {
        return null;
      }
      const content = await file.arrayBuffer();
      const contentBuffer = Buffer.from(content);

      // 3. Store locally before returning 
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
    let localPath;
    try {
      // Path traversal check is performed here
      localPath = this._getLocalPath(s3Key);
    } catch (e) {
      // Handle path traversal error from _getLocalPath
      console.error(`Rejected write attempt for key ${s3Key}:`, e.message);
      return;
    }
    
    // 1. Write to S3
    await this.client.write(s3Key, data);

    // 2. Write to local cache 
    await this._ensureDir(localPath);
    await fs.writeFile(localPath, data);
  }

  // --- Utility Operations ---
  async exists(s3Key) {
    let localPath;
    try {
      // Path traversal check is performed here
      localPath = this._getLocalPath(s3Key);
    } catch (e) {
      // Handle path traversal error from _getLocalPath
      console.error(`Rejected existence check for key ${s3Key}:`, e.message);
      return false;
    }
    
    try {
      const stats = await fs.stat(localPath);
      if (Date.now() - stats.mtimeMs < this.TTL_MS) return true;
    } catch (e) {
      // Not in cache, check S3
    }
    
    return this.client.file(s3Key).exists();
  }
}