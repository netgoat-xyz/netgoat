// Example proxy routes (in-memory, replace with DB logic as needed)
const proxies = [];

export function registerProxyRoutes(app) {
  // List all proxies
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
    reply.send(proxy);
  });

  // Update a proxy by ID
  app.put("/api/proxies/:id", async (request, reply) => {
    const idx = proxies.findIndex((p) => p.id === request.params.id);
    if (idx === -1) return reply.code(404).send({ error: "Not found" });
    proxies[idx] = { ...proxies[idx], ...request.body };
    reply.send(proxies[idx]);
  });

  // Delete a proxy by ID
  app.delete("/api/proxies/:id", async (request, reply) => {
    const idx = proxies.findIndex((p) => p.id === request.params.id);
    if (idx === -1) return reply.code(404).send({ error: "Not found" });
    const removed = proxies.splice(idx, 1)[0];
    reply.send(removed);
  });
}
