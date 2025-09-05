import mongoose from "mongoose";
import { Elysia } from "elysia";
import { parse } from "tldts";
import { Eta } from "eta";
import { request } from "undici";
import path from "path";
import Score from "../database/mongodb/schema/score.js";
import domains from "../database/mongodb/schema/domains.js";
import packageInfo from "../package.json" assert { type: "json" };
import logger from "../utils/logger.js";
import tracelet from "../utils/tracelet.js";

// --- App setup
const app = new Elysia();
const eta = new Eta({ views: path.join(process.cwd(), "views") });
const domainCache = new Map();

// --- Helpers
const getClientIp = (req) => {
  const xff = req.headers.get("x-forwarded-for");
  return xff
    ? xff.split(",")[0].trim()
    : req.headers.get("x-real-ip") || "unknown";
};

const logToLogDB = async (domain, subdomain, req, time, traceletId) => {
  try {
    const payload = {
        method: req.method,
      path: new URL(req.url).pathname,
        headers: Object.fromEntries(req.headers),
      ip: getClientIp(req),
      time: time.toISOString(),
      traceletId,
    };
    const sd = subdomain || "@"; // <-- default here
    await fetch(
      `${process.env.LogDB_instance}/api/${domain}/analytics?subdomain=${sd}`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      }
    );
  } catch (err) {
    logger.warn("Failed to send log to LogDB:", err);
  }
};

// --- Reverse proxy handler
app.all("/*", async ({ request: req, set }) => {
  const traceletId = tracelet(process.env.regionID);

  try {
    const host = req.headers.get("host")?.split(":")[0];
    if (!host) return new Response("Missing Host header", { status: 400 });

    const { domain, subdomain } = parse(host);
    if (!domain) return new Response("Invalid domain", { status: 400 });

    let domainData = domainCache.get(domain);
    if (!domainData) {
      domainData = await domains.findOne({ domain });
      if (domainData) domainCache.set(domain, domainData);
    }
    if (!domainData)
      return new Response("Domain not configured", { status: 404 });

    // pick first proxied target for example
    const urlPath = new URL(req.url).pathname;
    const requestedSub = subdomain || "@";

    // find proxied entry matching the slug (default "@" for root)
    const targetService =
      domainData.proxied?.find((p) => p.slug === requestedSub) ||
      domainData.proxied?.find((p) => p.slug === "@");

    if (!targetService) return new Response("Unknown host", { status: 502 });
    const ipAddress = getClientIp(req);

    // Banned IP check
    if (targetService.SeperateBannedIP?.some((b) => b.ip === ipAddress))
      return new Response("Forbidden", { status: 403 });

    // ACL check
    const userACL = targetService.SeperateACL?.find(
      (a) => a.user === ipAddress
    );
    if (userACL && !userACL.permission.access)
      return new Response("Access denied", { status: 403 });

    // Rate limit
    if (targetService.rateRules?.length) {
      const { requestsPerMinute } = targetService.rateRules[0];
      const cutoff = new Date(Date.now() - 60 * 1000);
      const recentReqs = await Score.countDocuments({
        ipAddress,
        createdAt: { $gte: cutoff },
      });
      if (recentReqs >= requestsPerMinute) {
        await domains.updateOne(
          { domain, "proxied.slug": targetService.slug },
          {
            $push: {
              "proxied.$.violations": {
                ip: ipAddress,
                reason: "Rate limit exceeded",
                path: new URL(req.url).pathname,
              },
            },
          }
        );
        return new Response("Rate limit exceeded", { status: 429 });
      }
    }

    // Forward to IP + PORT
    const protocol = targetService.SSL ? "https" : "http";
    const targetUrl = `${protocol}://${targetService.ip}:${targetService.port}${
      new URL(req.url).pathname
    }${new URL(req.url).search}`;

    const method = req.method;
    const hasBody = ["POST", "PUT", "PATCH", "DELETE"].includes(method);
    const reqBody = hasBody ? await req.text() : undefined;

    const upstream = await request(targetUrl, {
      method,
      headers: Object.fromEntries(req.headers),
      body: reqBody,
    });

    const headers = new Headers(upstream.headers);
    headers.set("x-tracelet-id", traceletId);
    headers.set("x-powered-by", `NetGoat ${packageInfo.version}`);
    headers.set(
      "Access-Control-Expose-Headers",
      "x-tracelet-id,x-powered-by,x-worker-id"
    );

    // logging
    await logToLogDB(domain, subdomain, req, new Date(), traceletId);

    const bodyText = await upstream.body.text();
    if ((upstream.headers["content-type"] || "").includes("text/html")) {
      const injectedScript = `
        <script src="https://unpkg.com/rrweb@latest/dist/rrweb.min.js"></script>
        <script src="https://api.netgoat.cloudable.dev/monitor.js"></script>
      `;
      return new Response(
        bodyText.replace("</body>", `${injectedScript}</body>`),
        { status: upstream.statusCode, headers }
      );
    }

    return new Response(bodyText, { status: upstream.statusCode, headers });
  } catch (err) {
    const html = await eta.render("error/500.ejs", {
      traceletId: tracelet(process.env.regionID),
      error: err.message,
    });
    return new Response(html, {
      status: 500,
      headers: { "Content-Type": "text/html" },
    });
  }
});

app.listen({ port: 80 });
app.listen({ port: 443 });

logger.info("Reverse proxy running on port 80");
