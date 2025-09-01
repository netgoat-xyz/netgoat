import vm from "vm";
import fetch from "node-fetch";
import tracelet from "../utils/tracelet.js";

// --- WAFScript Engine ---
class WAFEngine {
  constructor({ asnMap, ddosList }) {
    this.asnMap = asnMap || new Map();
    this.ddosList = ddosList || new Set();
  }

  async run(script, context = {}) {
    const sandbox = {
      console,
      Date,
      JSON,
      ...this.helpers(),
      ...context,
    };
    const scriptVM = new vm.Script(script);
    const ctx = vm.createContext(sandbox);
    return scriptVM.runInContext(ctx);
  }

  helpers() {
    return {
      // ASN Lookup
      getASN: (ip) => this.asnMap.get(ip) || null,

      // Check if IP is malicious
      isDDoSIP: (ip) => this.ddosList.has(ip),

      // Debug / Trace
      debugTrace: (msg) =>
        console.log(`[TRACE:${tracelet("waf")}]`, msg),

      // Time Helpers
      now: () => new Date(),
      epoch: () => Date.now(),
      formatTime: (d = new Date()) => d.toISOString(),

      // Request Info extractor
      getRequestInfo: (req) => ({
        method: req.method,
        url: req.url,
        headers: Object.fromEntries(req.headers),
        ip: req.headers["x-real-ip"] || req.ip || "unknown",
      }),

      // API Caller
      callAPI: async (url, opts = {}) => {
        const res = await fetch(url, opts);
        return res.json().catch(() => res.text());
      },
    };
  }
}