import fs from "fs";
import path from "path";
import { NodeVM } from "vm2";
import threatLists from "./threatLists.js";

export default class WAF {
  constructor() {
    this.rules = [];
  }

  // 👇 FIX APPLIED HERE: Two-stage loading process
  loadRuleFile(filePath) {
    const ruleFileContent = fs.readFileSync(filePath, "utf8");

    // --- Stage 1: Load the configuration object from the file (e.g., to get the name and .code property) ---
    const vmLoader = new NodeVM({
      console: "off",
      sandbox: {},
      require: false,
      timeout: 100,
    });
    
    let ruleConfig;
    try {
        // 1. Execute the file content to get the exported object (e.g., { default: { name: '...', code: '...' } })
        ruleConfig = vmLoader.run(ruleFileContent, filePath).default;
    } catch (e) {
        console.error(`[WAF Loader Error] Syntax: Failed to parse rule config file ${filePath}. Ensure 'export default { ... }' is valid.`, e.message);
        return;
    }

    // 2. Validate and extract the executable code string
    if (!ruleConfig || typeof ruleConfig.code !== 'string') {
        console.error(`[WAF Loader Error] Config: Rule file ${filePath} does not contain a valid 'code' property.`);
        return;
    }
    
    const codeToExecute = ruleConfig.code; 

    // --- Stage 2: Wrap and execute the code string safely ---
    
    // Create a new VM execution environment
    const vmExecutor = new NodeVM({
      console: "inherit", // Allow the rule's console.log to show up in the main console
      sandbox: {},
      require: false,
      timeout: 200, 
    });

    // Wrap ONLY the raw executable code string inside the function to avoid syntax errors
    const wrapped = `module.exports = async function(req, lists, helpers){ ${codeToExecute} }`;
    
    try {
        const func = vmExecutor.run(wrapped, filePath);
        this.rules.push({ func, file: filePath, name: ruleConfig.name || path.basename(filePath) });
    } catch (e) {
        console.error(`[WAF Executor Error] Runtime: Failed to wrap and run executable code in ${filePath}.`, e.message);
    }
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
        // If the error object contains 'action', it was intentionally thrown by a helper
        if (err && err.action) return err;
        // Otherwise, it's a real unexpected error
        console.error(`WAF Rule ${r.name} caused a runtime error:`, err);
        throw err;
      }
    }

    return { action: "allow" };
  }
}