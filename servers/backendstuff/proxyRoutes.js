import escapeHtml from "escape-html";
import fs from "fs";
import path from "path";
import domains from "../../database/mongodb/schema/domains.js";
import Users from "../../database/mongodb/schema/users.js";
import WAF from "../../utils/ruleScript.js";
import jwt from "jsonwebtoken";

const waf = new WAF();
const CERTS_DIR = path.join(process.cwd(), "database/certs");

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

  // ---------------- Domains ----------------
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

  // ---------------- Proxies ----------------
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
            { $pull: { proxied: { _id: body.proxyId } } }, // use the proxy's unique ID    
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

  // ---------------- WAF ----------------
  app.get("/api/waf/rules/:domain", async ({ params, headers }, reply) => {
    try {
      const token = headers.authorization?.split(" ")[1] || "";
      if (!token)
        throw { status: 401, message: "Invalid Authorization header" };
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

  app.post(
    "/api/waf/rules/:domain/:slug",
    async ({ params, body, headers }, reply) => {
      try {
        const token = headers.authorization?.split(" ")[1] || "";
        if (!token)
          throw { status: 401, message: "Invalid Authorization header" };
        const payload = jwt.verify(token, process.env.JWT_SECRET);

        await ensureDomainOwnership(payload.userId, params.domain);
        const updated = await domains.findOneAndUpdate(
          { domain: params.domain, "proxied.slug": params.slug },
          { $push: { "proxied.$.seperateRules": body } },
          { new: true }
        );
        return (
          updated ||
          reply.status(404).send({ error: "Domain or subdomain not found" })
        );
      } catch (err) {
        return reply
          .status(err.status || 500)
          .send({ success: false, error: err.message || err });
      }
    }
  );

  // ---------------- SSL ----------------
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
        const dir = path.join(
          CERTS_DIR,
          params.userId,
          params.domain,
          params.subdomain
        );
        if (!fs.existsSync(dir))
          return reply.status(404).send({ error: "No certs found" });

        const cert = fs.readFileSync(path.join(dir, "fullchain.pem"), "utf8");
        const key = fs.readFileSync(path.join(dir, "privkey.pem"), "utf8");
        return { cert, key };
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

        const dir = path.join(
          CERTS_DIR,
          params.userId,
          params.domain,
          params.subdomain
        );
        fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
        fs.writeFileSync(path.join(dir, "fullchain.pem"), body.cert);
        fs.writeFileSync(path.join(dir, "privkey.pem"), body.key, {
          mode: 0o600,
        });

        return reply.send({ success: true, path: dir });
      } catch (err) {
        return reply
          .status(err.status || 500)
          .send({ success: false, error: err.message || err });
      }
    }
  );

  // ---------------- Error Pages ----------------
  app.get(
    "/api/error-page/:domain/:code",
    async ({ params, headers }, reply) => {
      try {
        const token = headers.authorization?.split(" ")[1] || "";
        if (!token)
          throw { status: 401, message: "Invalid Authorization header" };

        const payload = jwt.verify(token, process.env.JWT_SECRET);

        await ensureDomainOwnership(payload.userId, params.domain);
        const filePath = path.join(
          process.cwd(),
          "views",
          "error",
          `${params.domain}_${params.code}.ejs`
        );
        if (!fs.existsSync(filePath))
          return reply.status(404).send({ error: "Not found" });
        return fs.readFileSync(filePath, "utf8");
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
        const filePath = path.join(
          process.cwd(),
          "views",
          "error",
          `${params.domain}_${params.code}.ejs`
        );
        fs.writeFileSync(filePath, body.html, { mode: 0o644 });
        return reply.send({ success: true });
      } catch (err) {
        return reply
          .status(err.status || 500)
          .send({ success: false, error: err.message || err });
      }
    }
  );

  // ---------------- Zero-Trust / Users ----------------
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

  // ---------------- WAF Script Upload ----------------
  app.post("/api/waf/upload", async ({ body, headers }, reply) => {
    try {
      const filePath = path.join(
        process.cwd(),
        "waf",
        "rules",
        `${body.name}.js`
      );
      fs.writeFileSync(filePath, body.code);
      await waf.loadRule(filePath);
      return reply.send({ success: true, file: filePath });
    } catch (err) {
      return reply
        .status(500)
        .send({ success: false, error: err.message || err });
    }
  });
}
