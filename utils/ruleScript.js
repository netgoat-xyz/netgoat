import fs from "fs";
import path from "path";
import { NodeVM } from "vm2";
import threatLists from "./threatLists.js";

export default class WAF {
  constructor() {
    this.rules = [];
  }

  loadRuleFile(filePath) {
    const code = fs.readFileSync(filePath, "utf8");
    const vm = new NodeVM({
      console: "off",
      sandbox: {},
      require: false,
      timeout: 200,
    });
    const wrapped = `module.exports = async function(req, lists, helpers){ ${code} }`;
    const func = vm.run(wrapped, filePath);
    this.rules.push({ func, file: filePath });
  }
  
  loadRule(filePath) {
    return this.loadRuleFile(filePath);
  }
  
  loadRulesDir(dir) {
    if (!fs.existsSync(dir)) return;
    for (const f of fs.readdirSync(dir)) {
      if (!f.endsWith(".js")) continue;
      this.loadRuleFile(path.join(dir, f));
    }
  }

  async checkRequest(req) {
    const helpers = {
      block: () => {
        throw { action: "block" };
      },
      allow: () => {
        throw { action: "allow" };
      },
      redirect: (url) => {
        throw { action: "redirect", url };
      },
      challenge: (type = "basic") => {
        throw { action: "challenge", type };
      },
    };

    for (const r of this.rules) {
      try {
        await r.func(req, threatLists, helpers);
      } catch (err) {
        if (err && err.action) return err;
        throw err;
      }
    }

    return { action: "allow" };
  }
}
