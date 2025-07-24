import { Elysia } from 'elysia';
import { parse } from 'tldts';
import { Eta } from 'eta';
import { request } from 'undici';
import path from 'path';
import Score from '../database/mongodb/schema/score';
import domains from '../database/mongodb/schema/domains';
import packageInfo from '../package.json';
import logger from '../utils/logger';

const app = new Elysia();
const eta = new Eta({ views: path.join(process.cwd(), 'views') });
const domainCache = new Map();

const getClientIp = (req) => {
  const xff = req.headers.get('x-forwarded-for');
  return xff ? xff.split(',')[0].trim() : req.headers.get('x-real-ip') || 'unknown';
};

const logToLogDB = async (domain, subdomain, req, time, traceletId) => {
  try {
    const payload = {
      method: req.method,
      path: new URL(req.url).pathname,
      headers: Object.fromEntries(req.headers),
      ip: getClientIp(req),
      time: time.toISOString(),
      traceletId
    };

    await fetch(`http://localhost:3010/api/${domain}/analytics?subdomain=${subdomain}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
  } catch (err) {
    logger.warn('Failed to send log to LogDB:', err);
  }
};

app.all('/*', async ({ request: req, set }) => {
  try {
    const host = req.headers.get('host')?.split(':')[0];
    if (!host) return new Response('Missing Host header', { status: 400 });

    const { domain, subdomain } = parse(host);
    if (!domain) return new Response('Invalid domain', { status: 400 });

    let domainData = domainCache.get(domain);
    if (!domainData) {
      domainData = await domains.findOne({ domain });
      if (domainData) domainCache.set(domain, domainData);
    }

    const target = domainData?.proxied;
    if (!target) return new Response('Unknown host', { status: 502 });

    const url = new URL(req.url, target);
    const ipAddress = getClientIp(req);

    const agg = await Score.aggregate([
      { $match: { ipAddress } },
      { $group: { _id: '$ipAddress', totalScore: { $sum: '$score' }, count: { $sum: 1 } } }
    ]);

    if (agg.length > 60) logger.warn(`IP ${ipAddress} has exceeded the score`);

    const method = req.method;
    const hasBody = ['POST', 'PUT', 'PATCH', 'DELETE'].includes(method);
    const reqBody = hasBody ? await req.text() : undefined;

    const upstream = await request(url.toString(), {
      method,
      headers: Object.fromEntries(req.headers),
      body: reqBody
    });

    const traceletId = tracelet(process.env.regionID);

    const headers = new Headers(upstream.headers);
    headers.set('x-tracelet-id', traceletId);
    headers.set('x-powered-by', `NetGoat ${packageInfo.version}`);
    headers.set('Access-Control-Expose-Headers', 'x-tracelet-id, x-powered-by, x-worker-id');

    await logToLogDB(domain, subdomain, req, new Date(), traceletId);

    try {
      const newScore = new Score({ ipAddress, score: 1 });
      await newScore.save();
    } catch (err) {
      logger.error('Failed to save request score:', err);
    }

    const contentType = upstream.headers['content-type'] || '';
    const bodyText = await upstream.body.text();

    if (contentType.includes('text/html')) {
      const injectedScript = `\n<!--\n<script src=\"https://unpkg.com/rrweb@latest/dist/rrweb.min.js\"></script>\n<script src=\"https://api.netgoat.cloudable.dev/monitor.js\"></script> -->\n`;
      const modifiedBody = bodyText.replace('</body>', `${injectedScript}</body>`);
      return new Response(modifiedBody, { status: upstream.statusCode, headers });
    }

    return new Response(bodyText, { status: upstream.statusCode, headers });
  } catch (err) {
    const html = await eta.render('error/500.ejs', { traceletId: tracelet(process.env.regionID), error: err.message });
    return new Response(html, { status: 500, headers: { 'Content-Type': 'text/html' } });
  }
});

app.listen({ port: 80 });
logger.info('Reverse proxy running on port 80');
