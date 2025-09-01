const vm = require("vm");
const fs = require("fs");
const threatLists = require("./threatLists");

class WAF {
  constructor() {
    this.rules = [];
  }

  loadRule(file) {
    const code = fs.readFileSync(file, "utf8");
    const script = new vm.Script(code);
    this.rules.push(script);
  }

  async checkRequest(req) {
    const sandbox = {
      req,
      block: () => { throw { action: "block" } },
      allow: () => { throw { action: "allow" } },
      redirect: (url) => { throw { action: "redirect", url } },
      challenge: () => { throw { action: "challenge" } },
      lists: threatLists,
      console,
      Buffer,
      Date,
      Math,
      setTimeout,
      clearTimeout,
      // NO require, NO fs, NO imports
    };
    const context = vm.createContext(sandbox);

    try {
      for (const rule of this.rules) {
        rule.runInContext(context);
      }
    } catch (err) {
      if (err.action) return err;
      throw err;
    }
    return { action: "allow" };
  }
}

module.exports = WAF;
