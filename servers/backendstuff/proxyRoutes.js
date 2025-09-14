// Example proxy routes (in-memory, replace with DB logic as needed)
import escapeHtml from "escape-html";
const proxies = [];
import domains from "../../database/mongodb/schema/domains.js";

export function registerProxyRoutes(app) {
  // Get Domain Data
  app.get("/api/domains/:domain", async ({ params }, reply) => {
    try {
      const domainData = await domains.findOne({ domain: params.domain });
<<<<<<< HEAD
      return domainData;
    } catch (err) {
      return new Response(
        JSON.stringify({ success: false, Error: err.message }),
        { status: 500 }
      );
    }
  });
  // Dynamically change

  app.post("/api/manage-proxy", async (request, reply) => {
    try {
      const domain = request.query.domain;
      if (!domain)
        return reply
          .status(400)
          .send({ success: false, Error: "Missing ?domain query parameter" });
      const domainData = await domains.findOne({ domain });
      if (!domainData)
        return reply
          .status(404)
          .send({ success: false, Error: "Domain not found" });

      const body = request.body;

      // make sure proxied conforms to schema
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

      const updatedDoc = await domains.findOneAndUpdate(
        { domain },
        { $push: { proxied: newProxy } },
        { new: true, upsert: true }
      );

      return new Response(
        JSON.stringify(
          {
            message: "New proxied record added",
            domain: updatedDoc,
            success: true,
          },
          null,
          2
        ),
        { status: 200, headers: { "Content-Type": "application/json" } }
      );
    } catch (err) {
      return new Response(
        JSON.stringify({ success: false, Error: err.message }),
        { status: 500 }
      );
    }
  });
=======
      return domainData
    } catch (err) {
      return new Response(JSON.stringify({ success: false, Error: err.message }), { status: 500 });
    }
  })
  // Dynamically change

app.post("/api/manage-proxy", async (request, reply) => {
  try {
    const domain = (request.query ).domain;
    if (!domain) return reply.status(400).send({ success: false, Error: "Missing ?domain query parameter" });

    const body = request.body

    // make sure proxied conforms to schema
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

    const updatedDoc = await domains.findOneAndUpdate(
      { domain },
      { $push: { proxied: newProxy } },
      { new: true, upsert: true }
    );

    return new Response(
      JSON.stringify(
        {
          message: "New proxied record added",
          domain: updatedDoc,
          success: true,
        },
        null,
        2
      ),
      { status: 200, headers: { "Content-Type": "application/json" } }
    );
  } catch (err) {
    return new Response(JSON.stringify({ success: false, Error: err.message }), { status: 500 });
  }
});

>>>>>>> 1e26c937094b9bb52e60e9b85f0514df46ed7c2d

  // Get all proxies
  app.get("/api/proxies", async (request, reply) => {
    reply.send(proxies);
  });

  // Create a new proxy
  app.post("/api/proxies", async (request, reply) => {
    const proxy = request.body;
    proxy.id = Date.now().toString();
    proxies.push(proxy);
    reply.code(201).send(proxy);
  });

  // Get a proxy by ID
  app.get("/api/proxies/:id", async (request, reply) => {
    const proxy = proxies.find((p) => p.id === request.params.id);
    if (!proxy) return reply.code(404).send({ error: "Not found" });
    const sanitizedProxy = Object.fromEntries(
      Object.entries(proxy).map(([key, value]) =>
        typeof value === "string" ? [key, escapeHtml(value)] : [key, value]
      )
    );
    reply.send(sanitizedProxy);
  });

  // Update a proxy by ID
  app.put("/api/proxies/:id", async (request, reply) => {
    const idx = proxies.findIndex((p) => p.id === request.params.id);
    if (idx === -1) return reply.code(404).send({ error: "Not found" });
    proxies[idx] = { ...proxies[idx], ...request.body };
    const sanitizedProxy = Object.fromEntries(
      Object.entries(proxies[idx]).map(([key, value]) =>
        typeof value === "string" ? [key, escapeHtml(value)] : [key, value]
      )
    );
    reply.send(sanitizedProxy);
  });

  // Delete a proxy by ID
  app.delete("/api/proxies/:id", async (request, reply) => {
    const idx = proxies.findIndex((p) => p.id === request.params.id);
    if (idx === -1) return reply.code(404).send({ error: "Not found" });
    const removed = proxies.splice(idx, 1)[0];
    const sanitizedRemoved = Object.fromEntries(
      Object.entries(removed).map(([key, value]) =>
        typeof value === "string" ? [key, escapeHtml(value)] : [key, value]
      )
    );
    reply.send(sanitizedRemoved);
  });
}
