import escapeHtml from "escape-html";
import domains from "../../database/mongodb/schema/domains.js";
import Users from "../../database/mongodb/schema/users.js";
import WAF from "../../utils/ruleScript.js";
import jwt from "jsonwebtoken";
import { S3Client } from "bun";

// --- Rate Limiting Setup ---
const RATE_LIMIT_WINDOW_MS = 60000; // 60 seconds
const RATE_LIMIT_MAX_REQUESTS = 50; // Max requests per IP per window
const requestTracker = new Map(); // Map<string (ip), { count: number, expiry: number }>

/**
 * Performs an in-memory rate limit check for a given IP address.
 * Uses X-Forwarded-For header for proxy-awareness.
 * @param {object} headers Request headers
 * @returns {{limited: boolean, resetTime?: number, limit?: number}}
 */
function checkRateLimit(headers) {
    // Note: The actual IP should ideally be retrieved from the request context (e.g., ctx.ip).
    // Using X-Forwarded-For as a proxy-aware fallback, or '127.0.0.1' otherwise.
    const ip = headers['x-forwarded-for']?.split(',')[0].trim() || '127.0.0.1';
    const now = Date.now();
    
    const record = requestTracker.get(ip);
    
    if (record && record.expiry > now) {
        // Window is still open
        if (record.count >= RATE_LIMIT_MAX_REQUESTS) {
            // Exceeded limit
            const resetTime = Math.ceil((record.expiry - now) / 1000);
            return {
                limited: true,
                resetTime: resetTime,
                limit: RATE_LIMIT_MAX_REQUESTS
            };
        }
        // Increment and continue
        record.count++;
        requestTracker.set(ip, record);
    } else {
        // First request or window expired, reset
        requestTracker.set(ip, {
            count: 1,
            expiry: now + RATE_LIMIT_WINDOW_MS
        });
    }
    
    return { limited: false };
}
// --- End Rate Limiting Setup ---

const waf = new WAF();

const WAFRules = new S3Client({
  accessKeyId: process.env.MINIO_ACCESS,
  secretAccessKey: process.env.MINIO_SECRET,
  bucket: "waf-rules",
  endpoint: process.env.MINIO_ENDPOINT, 
});

const SSLCerts = new S3Client({
  accessKeyId: process.env.MINIO_ACCESS,
  secretAccessKey: process.env.MINIO_SECRET,
  bucket: "ssl-certs",
  endpoint: process.env.MINIO_ENDPOINT, 
});

const UserGeneratedContent = new S3Client({
  accessKeyId: process.env.MINIO_ACCESS,
  secretAccessKey: process.env.MINIO_SECRET,
  bucket: "user-generated-content",
  endpoint: process.env.MINIO_ENDPOINT, 
});

export function registerProxyRoutes(app) {
  async function userOwnsDomain(userId, domainName) {
    const user = await Users.findById(userId).lean();
    if (!user) return false;
    return user.domains.some(
      (d) => d.name === domainName && d.status === "active"
    );
  }

  async function ensureDomainOwnership(userId, domainName) {
    const owns = await userOwnsDomain(userId, domainName);
    if (!owns) throw { status: 403, message: "You do not own this domain" };
  }

  app.get("/api/domains/:domain", async ({ params, headers }, reply) => {
    // --- Rate Limiting Check ---
    const limitCheck = checkRateLimit(headers);
    if (limitCheck.limited) {
        reply.header('Retry-After', limitCheck.resetTime);
        return reply.status(429).send({ 
            success: false, 
            error: `Rate limit exceeded. Try again in ${limitCheck.resetTime} seconds.`
        });
    }
    // --- End Rate Limiting Check ---
    
    try {
      const token = headers.authorization?.split(" ")[1] || "";
      if (!token)
        throw { status: 401, message: "Invalid Authorization header" };
      const payload = jwt.verify(token, process.env.JWT_SECRET);

      await ensureDomainOwnership(payload.userId, params.domain);
      const domainData = await domains.findOne({ domain: params.domain });
      return domainData;
    } catch (err) {
      return reply
        .status(err.status || 500)
        .send({ success: false, error: err.message || err });
    }
  });

  app.post("/api/manage-proxy", async ({ query, body, headers }, reply) => {
    // --- Rate Limiting Check ---
    const limitCheck = checkRateLimit(headers);
    if (limitCheck.limited) {
        reply.header('Retry-After', limitCheck.resetTime);
        return reply.status(429).send({ 
            success: false, 
            error: `Rate limit exceeded. Try again in ${limitCheck.resetTime} seconds.`
        });
    }
    // --- End Rate Limiting Check ---
    
    try {
      const domain = query.domain;
      const token = headers.authorization?.split(" ")[1] || "";
      if (!token)
        throw { status: 401, message: "Invalid Authorization header" };

      const payload = jwt.verify(token, process.env.JWT_SECRET);
      await ensureDomainOwnership(payload.userId, domain);

      const action = body.action || "add";

      let updatedDoc;

      switch (action) {
        case "add": {
          const newProxy = {
            domain: body.domain || "",
            port: body.port || 80,
            BlockCommonExploits: body.BlockCommonExploits ?? false,
            WS: body.WS ?? false,
            ip: body.ip || "",
            slug: body.slug || "@",
            SSL: body.SSL ?? false,
            SSLInfo: {
              localCert: body.SSLInfo?.localCert ?? false,
              certPaths: {
                PubKey: body.SSLInfo?.certPaths?.PubKey || "",
                PrivKey: body.SSLInfo?.certPaths?.PrivKey || "",
              },
            },
            seperateRules: body.seperateRules || [],
            SeperateACL: body.SeperateACL || [],
            SeperateBannedIP: body.SeperateBannedIP || [],
            rateRules: body.rateRules || [],
            violations: body.violations || [],
          };

          updatedDoc = await domains.findOneAndUpdate(
            { domain },
            { $push: { proxied: newProxy } },
            { new: true, upsert: true }
          );
          break;
        }

        case "edit": {
          if (!body.proxyId)
            throw { status: 400, message: "Missing proxyId for edit" };

          updatedDoc = await domains.findOneAndUpdate(
            { domain, "proxied._id": body.proxyId },
            {
              $set: {
                "proxied.$.port": body.port,
                "proxied.$.BlockCommonExploits": body.BlockCommonExploits,
                "proxied.$.WS": body.WS,
                "proxied.$.ip": body.ip,
                "proxied.$.slug": body.slug,
                "proxied.$.SSL": body.SSL,
                "proxied.$.SSLInfo": body.SSLInfo,
                "proxied.$.seperateRules": body.seperateRules,
                "proxied.$.SeperateACL": body.SeperateACL,
                "proxied.$.SeperateBannedIP": body.SeperateBannedIP,
                "proxied.$.rateRules": body.rateRules,
                "proxied.$.violations": body.violations,
              },
            },
            { new: true }
          );
          break;
        }

        case "delete": {
          updatedDoc = await domains.findOneAndUpdate(
            { domain },
            { $pull: { proxied: { _id: body.proxyId } } },    
            { new: true }
          );
          if (!updatedDoc)
            throw { status: 404, message: "Proxy not found for delete" };
          break;
        }

        default:
          throw { status: 400, message: "Invalid action" };
      }

      return reply.send({
        message: `Proxy ${action} successful`,
        domain: updatedDoc,
        success: true,
      });
    } catch (err) {
      return reply
        .status(err.status || 500)
        .send({ success: false, error: err.message || err });
    }
  });

  app.get("/api/waf/rules/:domain", async ({ params, headers }, reply) => {
    // --- Rate Limiting Check ---
    const limitCheck = checkRateLimit(headers);
    if (limitCheck.limited) {
        reply.header('Retry-After', limitCheck.resetTime);
        return reply.status(429).send({ 
            success: false, 
            error: `Rate limit exceeded. Try again in ${limitCheck.resetTime} seconds.`
        });
    }
    // --- End Rate Limiting Check ---
    
    try {
      const token = headers.authorization?.split(" ")[1] || "";
      if (!token) throw { status: 401, message: "Invalid Authorization header" };

      const payload = jwt.verify(token, process.env.JWT_SECRET);
      await ensureDomainOwnership(payload.userId, params.domain);

      const domainDoc = await domains.findOne({ domain: params.domain });
      return domainDoc?.proxied?.map((p) => p.seperateRules || []) || [];
    } catch (err) {
      return reply
        .status(err.status || 500)
        .send({ success: false, error: err.message || err });
    }
  });

app.post("/api/waf/rules/:domain", async (ctx, reply) => {
    // --- Rate Limiting Check ---
    const limitCheck = checkRateLimit(ctx.headers);
    if (limitCheck.limited) {
        reply.header('Retry-After', limitCheck.resetTime);
        return reply.status(429).send({ 
            success: false, 
            error: `Rate limit exceeded. Try again in ${limitCheck.resetTime} seconds.`
        });
    }
    // --- End Rate Limiting Check ---
    
    try {
      const params = { ...ctx.params }; // { domain: '...' }
      const headers = { ...ctx.headers };
      const body = ctx.body; // New rule configuration
      
      const token = headers.authorization?.split(" ")[1] || "";
      if (!token) throw { status: 401, message: "Invalid Authorization header" };
  
      const subdomainSlug = body.slug || "@";
      
      // Ensure the rule has a name for uniqueness tracking
      const ruleName = body.name?.replace(/\s+/g, "_").toLowerCase() || "unnamed_" + Date.now();
      body.name = ruleName;
      
      // 1. Authentication and Authorization Check
      const payload = jwt.verify(token, process.env.JWT_SECRET);
      await ensureDomainOwnership(payload.userId, params.domain);
  
      // 2. MongoDB Update: Find the document and attempt to update the rules array.
      // We use array filtering to either update an existing rule by name, or push a new one.

      const filter = { domain: params.domain, "proxied.slug": subdomainSlug };
      
      // 2a. Attempt to update an existing rule by name within the correct slug array
      let updatedDocument = await domains.findOneAndUpdate(
        { ...filter, "proxied.seperateRules.name": ruleName }, // Find if the rule name already exists
        { 
          // Use the array filter positional operator ($[...]) to set the new rule body
          $set: { 
            "proxied.$[proxiedEl].seperateRules.$[ruleEl]": body
          }
        },
        { 
          new: true,
          // Define the array filters to target the correct 'proxied' element and 'rule' element
          arrayFilters: [
            { "proxiedEl.slug": subdomainSlug }, 
            { "ruleEl.name": ruleName } 
          ]
        }
      );
      
      // 2b. If the rule did not exist (updatedDocument is null), push it as a new rule.
      if (!updatedDocument) {
        updatedDocument = await domains.findOneAndUpdate(
          filter, // Filter only by domain and slug
          { $push: { "proxied.$.seperateRules": body } },
          { new: true }
        );
      }
      
      if (!updatedDocument) {
        return reply.status(404).send({ 
          error: `Domain '${params.domain}' or subdomain slug '${subdomainSlug}' not found.` 
        });
      }

      // 3. Rule Consolidation (Align with Proxy's expectation)
      
      const targetProxyConfig = updatedDocument.proxied.find(p => p.slug === subdomainSlug);
      const allRules = targetProxyConfig?.seperateRules || [];
      
      let consolidatedCode = '';

      for (const ruleConfig of allRules) {
          if (ruleConfig.code) {
              // Stitch all raw rule code strings together into a single executable block
              consolidatedCode += `
// Rule: ${ruleConfig.name || 'Unnamed'} (Slug: ${subdomainSlug})
${ruleConfig.code}

`; // Add newlines for readability in the final script
          }
      }
      
      // 4. Wrap the consolidated code into the single 'export default' structure 
      //    that the WAF parser (checkRequestWithCode) expects.
      const finalExecutableScript = `
export default {
    "name": "${params.domain}-${subdomainSlug}-consolidated",
    "domain": "${params.domain}",
    "slug": "${subdomainSlug}",
    "code": ${JSON.stringify(consolidatedCode)},
    "description": "Consolidated WAF rules for ${params.domain}/${subdomainSlug}. Total rules: ${allRules.length}"
};
`;

      // 5. Upload the single consolidated script to the requested S3 path format
      // 🚩 S3 Key now includes the slug for organization.
      const s3RuleName = "consolidated"; 
      const s3Key = `custom-rules/${params.domain}/${subdomainSlug}/${s3RuleName}.js`; // e.g., custom-rules/semecom.com/@/consolidated.js
      await WAFRules.write(s3Key, finalExecutableScript);
      
      // 6. Optional: Invalidate Redis cache
      // 🚩 Redis key now includes the slug. The proxy must be updated to match!
      // await redis.del(`waf:rules:${params.domain}_${subdomainSlug}`); // Assumed redis client/import is available
  
      return reply.send({ 
          success: true, 
          message: `Consolidated ${allRules.length} rules and updated S3 for ${params.domain}/${subdomainSlug}.`,
          rulePath: s3Key 
      });
      
    } catch (err) {
        console.error("WAF Rule API Error:", err);
        return reply
            .status(err.status || 500)
            .send({ success: false, error: err.message || err });
    }
});

  app.get(
    "/api/ssl/:userId/:domain/:subdomain",
    async ({ params, headers }, reply) => {
      // --- Rate Limiting Check ---
      const limitCheck = checkRateLimit(headers);
      if (limitCheck.limited) {
          reply.header('Retry-After', limitCheck.resetTime);
          return reply.status(429).send({ 
              success: false, 
              error: `Rate limit exceeded. Try again in ${limitCheck.resetTime} seconds.`
          });
      }
      // --- End Rate Limiting Check ---
      
      try {
        const token = headers.authorization?.split(" ")[1] || "";
        if (!token)
          throw { status: 401, message: "Invalid Authorization header" };

        const payload = jwt.verify(token, process.env.JWT_SECRET);

        if (payload.userId.toString() !== params.userId)
          throw { status: 403, message: "Cannot access other users' certs" };

        await ensureDomainOwnership(payload.userId, params.domain);
        
        const certKey = `${params.userId}/${params.domain}/${params.subdomain}/fullchain.pem`;
        const privKey = `${params.userId}/${params.domain}/${params.subdomain}/privkey.pem`;

        try {
          const cert = await SSLCerts.file(certKey).text();
          const key = await SSLCerts.file(privKey).text();
          return { cert, key };
        } catch (e) {
          return reply.status(404).send({ error: "No certs found" });
        }
      } catch (err) {
        return reply
          .status(err.status || 500)
          .send({ success: false, error: err.message || err });
      }
    }
  );

  app.post(
    "/api/ssl/:userId/:domain/:subdomain",
    async ({ params, body, headers }, reply) => {
      // --- Rate Limiting Check ---
      const limitCheck = checkRateLimit(headers);
      if (limitCheck.limited) {
          reply.header('Retry-After', limitCheck.resetTime);
          return reply.status(429).send({ 
              success: false, 
              error: `Rate limit exceeded. Try again in ${limitCheck.resetTime} seconds.`
          });
      }
      // --- End Rate Limiting Check ---
      
      try {
        const token = headers.authorization?.split(" ")[1] || "";
        if (!token)
          throw { status: 401, message: "Invalid Authorization header" };

        const payload = jwt.verify(token, process.env.JWT_SECRET);

        if (payload.userId.toString() !== params.userId)
          throw { status: 403, message: "Cannot access other users' certs" };

        await ensureDomainOwnership(payload.userId, params.domain);

        const certKey = `${params.userId}/${params.domain}/${params.subdomain}/fullchain.pem`;
        const privKey = `${params.userId}/${params.domain}/${params.subdomain}/privkey.pem`;

        await SSLCerts.write(certKey, body.cert);
        await SSLCerts.write(privKey, body.key);

        return reply.send({ success: true, path: `${params.userId}/${params.domain}/${params.subdomain}` });
      } catch (err) {
        return reply
          .status(err.status || 500)
          .send({ success: false, error: err.message || err });
      }
    }
  );

  app.get(
    "/api/error-page/:domain/:code",
    async ({ params, headers }, reply) => {
      // --- Rate Limiting Check ---
      const limitCheck = checkRateLimit(headers);
      if (limitCheck.limited) {
          reply.header('Retry-After', limitCheck.resetTime);
          return reply.status(429).send({ 
              success: false, 
              error: `Rate limit exceeded. Try again in ${limitCheck.resetTime} seconds.`
          });
      }
      // --- End Rate Limiting Check ---
      
      try {
        const token = headers.authorization?.split(" ")[1] || "";
        if (!token)
          throw { status: 401, message: "Invalid Authorization header" };

        const payload = jwt.verify(token, process.env.JWT_SECRET);

        await ensureDomainOwnership(payload.userId, params.domain);
        
        const s3Key = `error-pages/${params.domain}_${params.code}.ejs`;
        try {
          const content = await UserGeneratedContent.file(s3Key).text();
          return content;
        } catch (e) {
           return reply.status(404).send({ error: "Not found" });
        }
      } catch (err) {
        return reply
          .status(err.status || 500)
          .send({ success: false, error: err.message || err });
      }
    }
  );

  app.post(
    "/api/error-page/:domain/:code",
    async ({ params, body, headers }, reply) => {
      // --- Rate Limiting Check ---
      const limitCheck = checkRateLimit(headers);
      if (limitCheck.limited) {
          reply.header('Retry-After', limitCheck.resetTime);
          return reply.status(429).send({ 
              success: false, 
              error: `Rate limit exceeded. Try again in ${limitCheck.resetTime} seconds.`
          });
      }
      // --- End Rate Limiting Check ---
      
      try {
        const token = headers.authorization?.split(" ")[1] || "";
        if (!token)
          throw { status: 401, message: "Invalid Authorization header" };

        const payload = jwt.verify(token, process.env.JWT_SECRET);

        await ensureDomainOwnership(payload.userId, params.domain);
        
        const s3Key = `error-pages/${params.domain}_${params.code}.ejs`;
        await UserGeneratedContent.write(s3Key, body.html);
        
        return reply.send({ success: true });
      } catch (err) {
        return reply
          .status(err.status || 500)
          .send({ success: false, error: err.message || err });
      }
    }
  );

  app.get("/api/users/:userId", async ({ params, headers }, reply) => {
    // --- Rate Limiting Check ---
    const limitCheck = checkRateLimit(headers);
    if (limitCheck.limited) {
        reply.header('Retry-After', limitCheck.resetTime);
        return reply.status(429).send({ 
            success: false, 
            error: `Rate limit exceeded. Try again in ${limitCheck.resetTime} seconds.`
        });
    }
    // --- End Rate Limiting Check ---
    
    try {
      const token = headers.authorization?.split(" ")[1] || "";
      if (!token)
        throw { status: 401, message: "Invalid Authorization header" };

      const payload = jwt.verify(token, process.env.JWT_SECRET);

      if (payload.userId.toString() !== params.userId)
        throw { status: 403, message: "Cannot view other users" };
      const u = await Users.findById(params.userId).lean();
      return u || reply.status(404).send({ error: "User not found" });
    } catch (err) {
      return reply
        .status(err.status || 500)
        .send({ success: false, error: err.message || err });
    }
  });

  app.post(
    "/api/users/:userId/integrations",
    async ({ params, body, headers }, reply) => {
      // --- Rate Limiting Check ---
      const limitCheck = checkRateLimit(headers);
      if (limitCheck.limited) {
          reply.header('Retry-After', limitCheck.resetTime);
          return reply.status(429).send({ 
              success: false, 
              error: `Rate limit exceeded. Try again in ${limitCheck.resetTime} seconds.`
          });
      }
      // --- End Rate Limiting Check ---
      
      try {
        const token = headers.authorization?.split(" ")[1] || "";
        if (!token)
          throw { status: 401, message: "Invalid Authorization header" };

        const payload = jwt.verify(token, process.env.JWT_SECRET);

        await ensureDomainOwnership(payload.userId, params.domain);

        if (payload.userId.toString() !== params.userId)
          throw { status: 403, message: "Cannot modify other users" };
        const updated = await Users.findByIdAndUpdate(
          params.userId,
          { $set: { integrations: body } },
          { new: true }
        );
        return updated || reply.status(404).send({ error: "User not found" });
      } catch (err) {
        return reply
          .status(err.status || 500)
          .send({ success: false, error: err.message || err });
      }
    }
  );

  app.post("/api/waf/upload", async ({ body, headers }, reply) => {
    // --- Rate Limiting Check ---
    const limitCheck = checkRateLimit(headers);
    if (limitCheck.limited) {
        reply.header('Retry-After', limitCheck.resetTime);
        return reply.status(429).send({ 
            success: false, 
            error: `Rate limit exceeded. Try again in ${limitCheck.resetTime} seconds.`
        });
    }
    // --- End Rate Limiting Check ---
    
    try {
      if (typeof body.name !== "string" || !body.name.match(/^[a-zA-Z0-9_-]+$/)) {
        return reply.status(400).send({ success: false, error: "Invalid rule name" });
      }
  
      const s3Key = `custom-rules/${body.name}.js`;
      await WAFRules.write(s3Key, body.code);
  
      return reply.send({ success: true, file: s3Key });
    } catch (err) {
      return reply.status(500).send({ success: false, error: err.message || err });
    }
  });
}