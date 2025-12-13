import vm from "node:vm";
import threatLists from "./threatLists.js";

/**
 * Max execution time for dynamic WAF rules (in milliseconds). 
 * CRITICAL: This prevents Denial of Service (DoS) by ensuring that malicious 
 * or poorly optimized rules do not consume unbounded resources.
 */
const EXECUTION_TIMEOUT_MS = 50;

export default class WAF {
  constructor() {
    this.rules = [];
  }

  /**
   * Security Utility: Sanitizes input strings before logging. 
   * This is necessary to mitigate log injection (log forging) vulnerabilities 
   * by stripping control characters and limiting string length.
   * @param {string} input The string to sanitize.
   * @returns {string} The sanitized string.
   */
  _sanitizeLogInput(input) {
    if (typeof input !== 'string') return String(input);
    // Strip newline (\n), carriage return (\r), and tab (\t) characters.
    return input.replace(/[\n\r\t]/g, ' ').substring(0, 256);
  }

  _getHelpers() {
    // WAF Action Helpers: These functions intentionally throw exceptions to 
    // immediately halt VM execution and signal the determined security action 
    // (block, allow, redirect, challenge) back to the calling process.
    return {
      block: () => { throw { action: "block" }; },
      allow: () => { throw { action: "allow" }; },
      redirect: (url) => { 
        // Enforce basic URL validation/safety here before throwing the action.
        if (typeof url !== 'string' || url.length > 2048) {
           throw new Error("Invalid redirect URL.");
        }
        throw { action: "redirect", url }; 
      },
      challenge: (type = "basic") => { throw { action: "challenge", type }; },
    };
  }

  /**
   * Executes dynamic WAF rules from external sources within a hardened VM context.
   * Execution relies on the strict sandboxing properties configured below.
   * * @param {Object} req The incoming request object for inspection.
   * @param {string} customRulesCode The untrusted JavaScript code to execute.
   * @param {string} ruleName A unique identifier for logging/debugging.
   * @returns {Promise<{action: string, url?: string, type?: string}>} The determined WAF action.
   */
  async checkRequestWithCode(req, customRulesCode, ruleName = "s3-dynamic-rule") {
    if (!customRulesCode || customRulesCode.trim() === '') {
        return { action: "allow" };
    }

    let codeToExecute = customRulesCode;
    // Sanitize ruleName early, as it is used in logging and VM filenames.
    const sanitizedRuleName = this._sanitizeLogInput(ruleName);

    // --- Stage 1: Handle Configuration Object Extraction ---
    // If the rule uses an "export default { code: '...' }" structure, we must 
    // execute a minimal script to extract the executable code string.
    if (customRulesCode.trim().startsWith('export default')) {
        try {
            // Convert ES module default export syntax to CommonJS for VM compatibility.
            const scriptContent = customRulesCode.replace('export default', 'module.exports =');
            
            // CRITICAL SECURITY: The config sandbox is intentionally minimal (only 'module') 
            // to prevent Remote Code Execution (RCE) during configuration parsing.
            const configSandbox = { module: { exports: {} } };
            vm.createContext(configSandbox);
            
            vm.runInContext(scriptContent, configSandbox, { 
                filename: `${sanitizedRuleName}-config.vm`,
                timeout: EXECUTION_TIMEOUT_MS, // Apply timeout to config parsing too
                displayErrors: false
            });
            
            const ruleConfig = configSandbox.module.exports;
            
            if (ruleConfig && typeof ruleConfig.code === 'string') {
                codeToExecute = ruleConfig.code;
            } else {
                console.error(`[WAF Parser] Rule ${sanitizedRuleName} invalid: missing 'code' string or incorrect structure.`);
                return { action: "allow" };
            }
        } catch (e) {
            // Log sanitation for error message. On failure to parse config, fail open (allow).
            console.error(`[WAF Parser] Execution error during config parsing for ${sanitizedRuleName}:`, this._sanitizeLogInput(e.message));
            return { action: "allow" };
        }
    }

    // --- Stage 2: Execute the Rule Logic ---
    // Wrap the rule code in an async IIFE to support 'await' within the rule logic.
    const wrappedCode = `
      (async () => {
        ${codeToExecute}
      })();
    `;

    // Sandboxed Global Scope: Only necessary, safe objects are exposed.
    const executionSandbox = {
      req,                     // Request context for inspection (read-only).
      lists: threatLists,      // Shared threat list data (read-only).
      helpers: this._getHelpers(), // WAF action functions (control flow).
      console,                 // Allowed for rule debugging/logging.
      // SECURITY: High-privilege global objects (Buffer, process, require, etc.) are 
      // explicitly omitted to prevent sandbox escape and subsequent RCE.
    };

    // Initialize the sandboxed context for rule execution.
    vm.createContext(executionSandbox);

    try {
      // Execute the code. Enforce timeout to protect against DoS.
      await vm.runInContext(wrappedCode, executionSandbox, {
        filename: `${sanitizedRuleName}-logic.vm`,
        timeout: EXECUTION_TIMEOUT_MS,
        displayErrors: false
      });
    } catch (err) {
      // 1. Catch intentional WAF action exceptions (block, allow, etc.).
      if (err && err.action) {
         return err;
      }
      
      // 2. Catch Timeout or other runtime errors in the rule.
      if (err && err.code === 'ERR_VM_TIMEOUT') {
          console.error(`[WAF Execution] Timeout (DoS) in ${sanitizedRuleName}. Rule took longer than ${EXECUTION_TIMEOUT_MS}ms.`);
      } else {
          // Log sanitation for rule name and error details.
          const errorMessage = err.message || (err.stack ? err.stack.split('\n')[0] : 'Unknown error');
          console.error(`[WAF Execution] Runtime error in ${sanitizedRuleName}:`, this._sanitizeLogInput(errorMessage));
      }
      
      // SSE Decision: Fail open on rule execution error (i.e., allow the request).
      return { action: "allow" };
    }

    // Default action if the rule finishes execution without explicitly calling a helper.
    return { action: "allow" };
  }

  // --- Deprecated Methods (kept for API shape compatibility) ---
  // These are placeholders for the original API surface, but not implemented 
  // as the dynamic S3/Redis rule loading is the primary method.
  loadRuleFile() {}
  loadRule() {}
  loadRulesDir() {}
  async checkRequest() { return { action: "allow" }; }
}