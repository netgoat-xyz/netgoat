import Fastify from "fastify";
import mongoose from "mongoose";
import Bun from "bun";
import User from "../database/mongodb/schema/users.js";
import jsonwebtoken from "jsonwebtoken";

const app = Fastify();
app.register(require('@fastify/cors'), { 
  // put your options here
})


app.get("/", async (request, reply) => {
  return { message: "Welcome to the backend Server!" };
});

app.post("/api/auth/register", async (request, reply) => {
  const { username, password, email } = request.body;

  if (!username || !password) {
    return reply.code(400).send({ error: "Username and password required" });
  }

  const existingUser = await User.findOne({ username });
  if (existingUser) {
    return reply.code(409).send({ error: "Username already exists" });
  }

  let hash = await Bun.password.hash(password);

  const user = new User({ username, password: hash, email, role: "user" });
  user
    .save()
    .then((doc) => {
      reply
        .code(201)
        .send({ success: true, reply: "User created successfully" });
    })
    .catch((err) => {
      console.error("Error creating user:", err);
      reply.code(500).send({ reply: "Internal server error", success: false });
    });
});

app.post("/api/auth/login", async (request, reply) => {
  const { username, email, password } = request.body;
  if ((!username && !email) || !password) {
    return reply
      .code(400)
      .send({ error: "Username or email and password required" });
  }
  try {
    const user = await User.findOne({ email: email });
    if (!user) return reply.code(401).send({ error: "Invalid user" });
    if (!await Bun.password.verify(password, user.password)) {
      return reply.code(401).send({ error: "Invalid credentials" });
    }

    let jwttoken = jsonwebtoken.sign(
      {
        userId: user._id,
        username: user.username,
        email: user.email,
        role: user.role,
      },
      process.env.JWT_SECRET
    );

    const requires2FA =
      user.integrations &&
      user.integrations.twofa &&
      user.integrations.twofa.enabled;

    if (requires2FA) {
      return reply.send({ requires2FA: true, jwt: jwttoken });
    }

    reply.send({ requires2FA: false, jwt: jwttoken });
  } catch (err) {
    console.error(err)
    reply.code(500).send({ error: err.message });
  }
});

app.get("/monitor.js", async (request, reply) => {
  reply.type("application/javascript");
  return `
      (() => {
        const sessionId = localStorage._sessionId ||= crypto.randomUUID();
        const events = [];

        rrweb.record({
          emit(event) {
            events.push(event);
          },
          recordCanvas: true
        });

        window.addEventListener('error', e => {
          events.push({
            type: 'custom',
            timestamp: Date.now(),
            data: {
              tag: 'error',
              message: e.message,
              stack: e.error?.stack || null
            }
          });
        });

        setInterval(() => {
          if (!events.length) return;

          const payload = {
            sessionId,
            timestamp: Date.now(),
            events: events.splice(0, events.length)
          };

          navigator.sendBeacon('/__monitor/replay', JSON.stringify(payload));
        }, 3000);
      })();
    `;
});

// --- Proxy API routes ---

// Example: Replace with your actual DB/model import
// import { proxies } from '../database/yourModel';
const proxies = [];

// List all proxies
app.get("/api/proxies", async (request, reply) => {
  // Replace with DB findAll
  reply.send(proxies);
});

// Create a new proxy
app.post("/api/proxies", async (request, reply) => {
  const proxy = request.body;
  // Replace with DB insert
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

app.listen({ port: 3001 }, (err, address) => {
  if (err) {
    console.error(err);
    process.exit(1);
  }
  logger.info(`Backend loaded at ${address}`);
});
