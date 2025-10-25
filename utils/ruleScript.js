// waf.js (ESM)
import fs from "fs";
import vm from "vm";
import path from "path";
import threatLists from "./threatLists.js"; // must export JS objects

export default class WAF {
  constructor() {
    this.rules = [];
  }

  loadRuleFile(filePath) {
    const code = fs.readFileSync(filePath, "utf8");
    const script = new vm.Script(code, { filename: path.basename(filePath) });
    this.rules.push({ script, file: filePath });
  }

  loadRulesDir(dir) {
    if (!fs.existsSync(dir)) return;
    for (const f of fs.readdirSync(dir)) {
      if (!f.endsWith(".js")) continue;
      this.loadRuleFile(path.join(dir, f));
    }
  }

  /**
   * req: object shaped { method, url, headers: Map or Object, ip, body }
   * returns: { action: "allow" } or { action: "block"|'redirect'|'challenge', ... }
   */
  async checkRequest(req) {
    const sandbox = {
      req,
      lists: threatLists,
      // action helpers throw a sentinel that we catch
      block: () => { throw { action: "block" }; },
      allow: () => { throw { action: "allow" }; },
      redirect: (url) => { throw { action: "redirect", url }; },
      challenge: (type = "basic") => { throw { action: "challenge", type }; },

      // utilities that are safe to expose
      console: console,
      Buffer,
      Date,
      Math,
      setTimeout,
      clearTimeout,
    };

    const context = vm.createContext(sandbox, { name: "waf-context" });

    try {
      for (const r of this.rules) {
        r.script.runInContext(context, { timeout: 50 }); // per-rule micro timeout
      }
    } catch (err) {
      if (err && err.action) return err;
      // rethrow unexpected errors (so operator notices)
      throw err;
    }
    return { action: "allow" };
  }
}
