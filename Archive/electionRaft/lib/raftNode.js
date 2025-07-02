import Fastify from 'fastify';
import axios from 'axios';
import jwt from 'jsonwebtoken';
import os from 'os';
import { execSync } from 'child_process';
import { write, findOne } from '../../database/clusterDB.js';

export default async function startRaftNode({ nodeId, port, peers, shardManagerURL }) {
  const app = Fastify();
  const JWT_SECRET = process.env.JWT_SECRET || 'supersecretjwtkey';
  const MIN_SCORE_THRESHOLD = 0.45;

  let state = 'follower';
  let currentTerm = 0;
  let votedFor = null;
  let leaderId = null;
  let electionTimeout = null;
  let heartbeatInterval = null;

  let cachedScore = null;
  let lastScoreTime = 0;

  async function getHealthScore() {
    const now = Date.now();
    if (cachedScore && now - lastScoreTime < 5000) return cachedScore;

    const cpuLoad = os.loadavg()[0];
    const freeMemRatio = os.freemem() / os.totalmem();
    const latency = await avgPing();
    const cpuPercent = parseFloat(execSync('ps -p ' + process.pid + ' -o %cpu').toString().split('\n')[1]) || 0;

    const score =
      (1 - latency / 1000) * 0.4 +
      (1 - cpuPercent / 100) * 0.2 +
      freeMemRatio * 0.2 +
      (1 - cpuLoad / os.cpus().length) * 0.2;

    cachedScore = Number(score.toFixed(3));
    lastScoreTime = now;
    return cachedScore;
  }

  async function avgPing() {
    let total = 0;
    let count = 0;
    for (const peer of peers) {
      const start = Date.now();
      try {
        await axios.get(`${peer}/status`, { timeout: 1000 });
        total += Date.now() - start;
        count++;
      } catch {}
    }
    return count ? total / count : 999;
  }

  async function registerSelf() {
  try {
    const res = await axios.post(`${shardManagerURL}/register`, {
      url: `http://localhost:${port}`,
      key: process.env.REGISTER_KEY
    });
    const token = res.data.token;
    await write('nodes', { _id: nodeId, token, url: `http://localhost:${port}`, lastSeen: Date.now() });
    return token;
  } catch (err) {
    console.error(`[${nodeId}] Failed to register with shard manager:`, err.message);
    return null;
  }
}
  app.addHook('preHandler', async (req, reply) => {
    if (["/vote", "/heartbeat"].includes(req.routerPath)) {
      const auth = req.headers.authorization;
      if (!auth?.startsWith("Bearer ")) return reply.code(401).send({ error: "No token" });
      try {
        const token = auth.split(" ")[1];
        const peerNode = await findOne("nodes", (n) => n.token === token);
        if (!peerNode) throw new Error("Invalid token");
        req.nodeId = peerNode._id;
      } catch {
        return reply.code(403).send({ error: "Invalid token" });
      }
    }
  });

  function resetElectionTimeout() {
    clearTimeout(electionTimeout);
    electionTimeout = setTimeout(startElection, 150 + Math.random() * 150);
  }

  function becomeLeader() {
    state = 'leader';
    leaderId = nodeId;
    console.log(`[${nodeId}] Became Leader!`);
    startHeartbeat();
    startScoreMonitor();
  }

  function startHeartbeat() {
    clearInterval(heartbeatInterval);
    heartbeatInterval = setInterval(async () => {
      const score = await getHealthScore();
      const allNodes = await findOne('nodes', () => true) || [];
      for (const peer of allNodes) {
        if (peer._id === nodeId) continue;
        axios.post(`${peer.url}/heartbeat`, {
          term: currentTerm,
          leaderId: nodeId,
          score
        }, {
          headers: { Authorization: `Bearer ${peer.token}` }
        }).catch(() => {});
      }
    }, 100);
  }

  function startScoreMonitor() {
    setInterval(async () => {
      if (state !== 'leader') return;
      const score = await getHealthScore();
      if (score < MIN_SCORE_THRESHOLD) {
        console.log(`[${nodeId}] Stepping down, score: ${score}`);
        state = 'follower';
        leaderId = null;
        resetElectionTimeout();
      }
    }, 3000);
  }

  async function startElection() {
    currentTerm++;
    state = 'candidate';
    votedFor = nodeId;
    let votes = 1;
    const score = await getHealthScore();

    const nodes = await findOne('nodes', () => true) || [];
    for (const peer of nodes) {
      if (peer._id === nodeId) continue;
      try {
        const res = await axios.post(`${peer.url}/vote`, {
          term: currentTerm,
          candidateId: nodeId,
          score
        }, {
          headers: { Authorization: `Bearer ${peer.token}` }
        });
        if (res.data.voteGranted) {
          votes++;
          if (votes > nodes.length / 2 && state === 'candidate') {
            becomeLeader();
            break;
          }
        }
      } catch {}
    }
    resetElectionTimeout();
  }

  app.post('/vote', async (req, reply) => {
    const { term, candidateId, score } = req.body;
    if (term > currentTerm) {
      currentTerm = term;
      state = 'follower';
      const myScore = await getHealthScore();
      const granted = score >= myScore;
      if (granted) votedFor = candidateId;
      resetElectionTimeout();
      return reply.send({ voteGranted: granted });
    }
    reply.send({ voteGranted: false });
  });

  app.post('/heartbeat', (req, reply) => {
    const { term, leaderId: lid } = req.body;
    if (term >= currentTerm) {
      currentTerm = term;
      state = 'follower';
      leaderId = lid;
      resetElectionTimeout();
    }
    reply.send();
  });

  app.get('/status', async (_, reply) => {
    const score = await getHealthScore();
    reply.send({
      nodeId,
      state,
      currentTerm,
      leaderId,
      leaderScore: leaderId === nodeId ? score : 0
    });
  });

  await registerSelf();
  app.listen({ port }, () => {
    console.log(`[${nodeId}] Raft started on :${port}`);
    resetElectionTimeout();
  });
}
