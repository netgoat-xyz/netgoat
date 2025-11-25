import fs from "fs";
import path from "path";
import vm from "node:vm"; // Use native Node VM instead of vm2
import threatLists from "./threatLists.js";

export default class WAF {
  constructor() {
    this.rules = [];
  }

  _getHelpers() {
    return {
      block: () => { throw { action: "block" }; },
      allow: () => { throw { action: "allow" }; },
      redirect: (url) => { throw { action: "redirect", url }; },
      challenge: (type = "basic") => { throw { action: "challenge", type }; },
    };
  }

  /**
   * Executes dynamic WAF rules fetched from S3/Redis.
   * Uses native node:vm for Bun compatibility.
   */
  async checkRequestWithCode(req, customRulesCode, ruleName = "s3-dynamic-rule") {
    if (!customRulesCode || customRulesCode.trim() === '') {
        return { action: "allow" };
    }

    const helpers = this._getHelpers();
    let codeToExecute = customRulesCode;

    // --- Stage 1: Handle Configuration Objects (export default) ---
    if (customRulesCode.trim().startsWith('export default')) {
        try {
            // Convert "export default" to CommonJS style for VM execution
            const scriptContent = customRulesCode.replace('export default', 'module.exports =');
            
            // Create a temporary context to parse the config
            const configSandbox = { module: { exports: {} }, console };
            vm.createContext(configSandbox);
            vm.runInContext(scriptContent, configSandbox);
            
            const ruleConfig = configSandbox.module.exports;
            
            if (ruleConfig && typeof ruleConfig.code === 'string') {
                codeToExecute = ruleConfig.code;
            } else {
                console.error(`[WAF Parser] Rule ${ruleName} invalid: missing 'code' string.`);
                return { action: "allow" };
            }
        } catch (e) {
            console.error(`[WAF Parser] Syntax error in config for ${ruleName}:`, e.message);
            return { action: "allow" };
        }
    }

    // --- Stage 2: Execute the Rule Logic ---
    // We wrap the code in an async IIFE to allow top-level returns (via throwing) and await.
    const wrappedCode = `
      (async () => {
        try {
          ${codeToExecute}
        } catch (e) {
          throw e;
        }
      })();
    `;

    const sandbox = {
      req,
      lists: threatLists,
      helpers,
      console, // Allow rules to log
      Buffer,  // Allow buffer operations
      module: {},
      exports: {}
    };

    vm.createContext(sandbox);

    try {
      await vm.runInContext(wrappedCode, sandbox);
    } catch (err) {
      // 1. Catch intentional WAF actions (Block/Allow/Redirect)
      if (err && err.action) return err;

      // 2. Catch actual runtime errors in the rule
      console.error(`[WAF Execution] Error in ${ruleName}:`, err.message);
      // Optional: Uncomment to block on error, currently fails open (allow)
      // throw err; 
    }

    return { action: "allow" };
  }

  // --- Deprecated Methods (kept for API shape compatibility) ---
  loadRuleFile() {}
  loadRule() {}
  loadRulesDir() {}
  async checkRequest() { return { action: "allow" }; }
}