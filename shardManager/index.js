import 'dotenv/config'

import Fastify from 'fastify';
import axios from 'axios';
import {QuickDB} from 'quick.db';
import jwt from 'jsonwebtoken';
import logger from './logger.js';
import bodyParser from 'body-parser';
const fastify = Fastify();
const JWT_SECRET = process.env.JWT_SECRET;
const REGISTER_KEY = process.env.REGISTER_KEY;

fastify.addContentTypeParser('application/json', { parseAs: 'string' }, function (req, body, done) {
  try {
    const json = JSON.parse(body);
    done(null, json);
  } catch (err) {
    done(err, undefined);
  }
});

const db = new QuickDB({
  file: './shard-manager.sqlite',
});

await db.set('nodes', {}); // ensure init
await db.set('assignments', {}); // ensure init

fastify.post('/register', async (req, reply) => {
  const { url, key } = req.body;
  if (key !== REGISTER_KEY) return reply.code(403).send({ error: 'Invalid global key' });

  const nodes = (await db.get('nodes')) || {};
  let bleh = nodes[url]
  if (!url || nodes[url]) return reply.send({success: true, bleh});

  const token = jwt.sign({ url }, JWT_SECRET);
  nodes[url] = token;
  await db.set('nodes', nodes);
  logger.info(`[ShardManager] Registered new node: ${url}`);
  reply.send({ success: true, token });
});


fastify.post('/move-shards', async (req, reply) => {
  const from = req.nodeUrl;
  logger.info(`[ShardManager] Rebalancing from: ${from}`);

  const nodes = Object.keys((await db.get('nodes')) || {});
  const nodeScores = await Promise.all(nodes.map(async (url) => {
    try {
      const res = await axios.get(`${url}/status`);
      return { url, score: res.data.leaderScore || 0 };
    } catch {
      return { url, score: 0 };
    }
  }));

  const targets = nodeScores
    .filter(n => n.url !== from && n.score > 0.5)
    .sort((a, b) => b.score - a.score);

  if (!targets.length) return reply.status(500).send('No good node');

  const assignments = (await db.get('assignments')) || {};
  const toMove = Object.entries(assignments).filter(([_, owner]) => owner === from);

  for (const [shardId] of toMove) {
    const target = targets[Math.floor(Math.random() * targets.length)].url;
    logger.info(`[ShardManager] Moving ${shardId} → ${target}`);
    assignments[shardId] = target;
    try {
      await axios.post(`${target}/shard/start`, { shardId });
    } catch {
      logger.warn(`[ShardManager] Failed to notify ${target} about ${shardId}`);
    }
  }

  await db.set('assignments', assignments);
  reply.send({ success: true });
});

fastify.get('/assignments', async (_, reply) => {
  reply.send(await db.get('assignments') || {});
});

fastify.listen({ port: 4000 }, (err) => {
  if (err) {
    console.error(err);
    process.exit(1);
  }
  logger.success('[ShardManager] Listening on 4000');
});
