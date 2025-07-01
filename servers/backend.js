import Fastify from 'fastify';
import mongoose from 'mongoose';

const app = Fastify()

app.get('/', async (request, reply) => {
    return { message: 'Welcome to the backend Server!' };
});

app.get('/monitor.js', async (request, reply) => {
    reply.type('application/javascript');
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
app.get('/api/proxies', async (request, reply) => {
    // Replace with DB findAll
    reply.send(proxies);
});

// Create a new proxy
app.post('/api/proxies', async (request, reply) => {
    const proxy = request.body;
    // Replace with DB insert
    proxy.id = Date.now().toString();
    proxies.push(proxy);
    reply.code(201).send(proxy);
});

// Get a proxy by ID
app.get('/api/proxies/:id', async (request, reply) => {
    const proxy = proxies.find(p => p.id === request.params.id);
    if (!proxy) return reply.code(404).send({ error: 'Not found' });
    reply.send(proxy);
});

// Update a proxy by ID
app.put('/api/proxies/:id', async (request, reply) => {
    const idx = proxies.findIndex(p => p.id === request.params.id);
    if (idx === -1) return reply.code(404).send({ error: 'Not found' });
    proxies[idx] = { ...proxies[idx], ...request.body };
    reply.send(proxies[idx]);
});

// Delete a proxy by ID
app.delete('/api/proxies/:id', async (request, reply) => {
    const idx = proxies.findIndex(p => p.id === request.params.id);
    if (idx === -1) return reply.code(404).send({ error: 'Not found' });
    const removed = proxies.splice(idx, 1)[0];
    reply.send(removed);
});

// --- User Schema ---
const userSchema = new mongoose.Schema({
  username: { type: String, required: true, unique: true },
  password: { type: String, required: true },
});
const User = mongoose.models.User || mongoose.model('User', userSchema);

// --- Mongoose Connection ---
const mongoUrl = process.env.MONGO_URL || 'mongodb://localhost:27017/netgoat';
if (mongoose.connection.readyState === 0) {
  mongoose.connect(mongoUrl, { useNewUrlParser: true, useUnifiedTopology: true });
}

// --- Auth API routes ---
app.post('/api/auth/register', async (request, reply) => {
  const { username, password } = request.body;
  if (!username || !password) {
    return reply.code(400).send({ error: 'Username and password required' });
  }
  try {
    const exists = await User.findOne({ username });
    if (exists) return reply.code(409).send({ error: 'Username already exists' });
    const user = await User.create({ username, password });
    reply.code(201).send({ id: user._id, username: user.username });
  } catch (err) {
    reply.code(500).send({ error: err.message });
  }
});

app.post('/api/auth/login', async (request, reply) => {
  const { username, password } = request.body;
  if (!username || !password) {
    return reply.code(400).send({ error: 'Username and password required' });
  }
  try {
    const user = await User.findOne({ username, password });
    if (!user) return reply.code(401).send({ error: 'Invalid credentials' });
    reply.send({ id: user._id, username: user.username });
  } catch (err) {
    reply.code(500).send({ error: err.message });
  }
});

app.listen({ port: 3001 }, (err, address) => {
    if (err) {
        console.error(err);
        process.exit(1);
    }
    logger.info(`Backend loaded at ${address}`);
});