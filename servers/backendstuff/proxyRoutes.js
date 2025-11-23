import escapeHtml from "escape-html";
import domains from "../../database/mongodb/schema/domains.js";
import Users from "../../database/mongodb/schema/users.js";
import WAF from "../../utils/ruleScript.js";
import jwt from "jsonwebtoken";
import { S3Client } from "bun";

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
    try {
      const params = { ...ctx.params };
      const headers = { ...ctx.headers };
      const body = ctx.body; 
      
      const token = headers.authorization?.split(" ")[1] || "";
      if (!token) throw { status: 401, message: "Invalid Authorization header" };
  
      const payload = jwt.verify(token, process.env.JWT_SECRET);
      await ensureDomainOwnership(payload.userId, params.domain);
  
      const updated = await domains.findOneAndUpdate(
        { domain: params.domain, "proxied.slug": params.slug },
        { $push: { "proxied.$.seperateRules": body } },
        { new: true }
      );
      if (!updated)
        return reply.status(404).send({ error: "Domain or subdomain not found" });
  
      const subdir = params.slug === "@" ? "@" : params.slug;
      const ruleName = body.name?.replace(/\s+/g, "_").toLowerCase() || "unnamed_rule";
      const s3Key = `${params.domain}/${subdir}/${ruleName}.js`;
      const ruleContent = `export default ${JSON.stringify(body, null, 2)};\n`;
  
      await WAFRules.write(s3Key, ruleContent);
  
      return reply.send({ success: true, updated, rulePath: s3Key });
    } catch (err) {
      return reply
        .status(err.status || 500)
        .send({ success: false, error: err.message || err });
    }
  });
  
  app.get(
    "/api/ssl/:userId/:domain/:subdomain",
    async ({ params, headers }, reply) => {
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