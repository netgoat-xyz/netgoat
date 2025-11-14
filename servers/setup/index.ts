import { serve } from "bun";
import fs from "fs";
import path from "path";

const outDir = path.join(import.meta.dir, "out");
let requestCount = 0;
serve({
  port: 3000,
  async fetch(req) {
        requestCount++;
            console.log("New request:", requestCount, req.url);
    const url = new URL(req.url);
    let filePath = path.join(outDir, url.pathname === "/" ? "index.html" : url.pathname);

    // If the path is a directory, serve index.html inside
    if (fs.existsSync(filePath) && fs.statSync(filePath).isDirectory()) {
      filePath = path.join(filePath, "index.html");
    }

    // SPA fallback: if file doesn’t exist, fallback to index.html
    if (!fs.existsSync(filePath) || !fs.statSync(filePath).isFile()) {
      filePath = path.join(outDir, "index.html");
    }

    // Final check
    if (!fs.existsSync(filePath) || !fs.statSync(filePath).isFile()) {
      return new Response("404 - Not Found", { status: 404 });
    }

    const ext = path.extname(filePath).toLowerCase();
    const mimeTypes: Record<string, string> = {
      ".html": "text/html",
      ".js": "application/javascript",
      ".css": "text/css",
      ".json": "application/json",
      ".woff": "font/woff",
      ".woff2": "font/woff2",
      ".ttf": "font/ttf",
      ".png": "image/png",
      ".jpg": "image/jpeg",
      ".jpeg": "image/jpeg",
      ".gif": "image/gif",
      ".svg": "image/svg+xml",
    };


    return new Response(Bun.file(filePath), {
      headers: {
        "Content-Type": mimeTypes[ext] || "application/octet-stream",
      },
    });
  },
});

console.log("Serving Next.js static export on http://localhost:3000");
