import User from "../../database/mongodb/schema/users.js";
import Domain from "../../database/mongodb/schema/domains.js";
import jsonwebtoken from "jsonwebtoken";
import Bun from "bun";
import { S3Client } from "bun";
import { queryLogs, getLogStats } from "../../utils/clickhouseClient.js";

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
export function registerRoutes(app) {
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
    try {
      await user.save();
      reply
        .code(201)
        .send({ success: true, reply: "User created successfully" });
    } catch (err) {
      console.error("Error creating user:", err);
      reply.code(500).send({ reply: "Internal server error", success: false });
    }
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
      if (!(await Bun.password.verify(password, user.password))) {
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
      console.log(jwttoken);
      const requires2FA =
        user.integrations &&
        user.integrations.twofa &&
        user.integrations.twofa.enabled;
      if (requires2FA) {
        return reply.send({ requires2FA: true, jwt: jwttoken });
      }
      reply.send({ requires2FA: false, jwt: jwttoken });
    } catch (err) {
      console.error(err);
      reply.code(500).send({ error: err.message });
    }
  });

  app.post("/api/profile/update", async (request, reply) => {
    try {
      const authHeader = request.headers.authorization;
      if (!authHeader || !authHeader.startsWith("Bearer ")) {
        return reply
          .code(401)
          .send({ error: "Missing or invalid authorization header" });
      }

      const token = authHeader.slice(7);
      let decoded;
      try {
        decoded = jsonwebtoken.verify(token, process.env.JWT_SECRET);
      } catch (err) {
        return reply.code(401).send({ error: "Invalid or expired token" });
      }

      const userId = decoded.userId;
      const { firstName, lastName, email, username, timezone } = request.body;

      if (!firstName && !lastName && !email && !username && !timezone) {
        return reply
          .code(400)
          .send({ error: "At least one field must be provided" });
      }

      if (username) {
        const existingUser = await User.findOne({
          username,
          _id: { $ne: userId },
        });
        if (existingUser) {
          return reply.code(409).send({ error: "Username already taken" });
        }
      }

      if (email) {
        const existingUser = await User.findOne({
          email,
          _id: { $ne: userId },
        });
        if (existingUser) {
          return reply.code(409).send({ error: "Email already in use" });
        }
      }

      const updateData = {};
      if (firstName) updateData.firstName = firstName;
      if (lastName) updateData.lastName = lastName;
      if (email) updateData.email = email;
      if (username) updateData.username = username;
      if (timezone) updateData.timezone = timezone;

      const user = await User.findByIdAndUpdate(userId, updateData, {
        new: true,
      });
      if (!user) {
        return reply.code(404).send({ error: "User not found" });
      }

      reply.send({
        success: true,
        message: "Profile updated successfully",
        user: {
          userId: user._id,
          username: user.username,
          email: user.email,
          firstName: user.firstName || "",
          lastName: user.lastName || "",
          timezone: user.timezone || "",
          role: user.role,
          createdAt: user.createdAt,
          updatedAt: user.updatedAt,
        },
      });
    } catch (err) {
      console.error("Error updating profile:", err);
      reply.code(500).send({ error: "Internal server error" });
    }
  });

  app.get("/avatars/:filename", async (request, reply) => {
    const { filename } = request.params;

    try {
      const file = UserGeneratedContent.file(filename);

      // Bun S3 file objects have a .stream() method
      const exists = await file.exists();
      if (!exists) return reply.code(404).send("Not found");

      reply.header("Content-Type", file.type || "image/png");
      reply.header("Cache-Control", "public, max-age=86400"); // Cache for 1 day

      return file.stream();
    } catch (err) {
      return reply.code(404).send("Avatar not found");
    }
  });

  app.post("/api/profile/avatar", async (request, reply) => {
    try {
      const authHeader = request.headers.authorization;
      if (!authHeader || !authHeader.startsWith("Bearer ")) {
        return reply
          .code(401)
          .send({ error: "Missing or invalid authorization header" });
      }

      const token = authHeader.slice(7);
      let decoded;
      try {
        decoded = jsonwebtoken.verify(token, process.env.JWT_SECRET);
        console.log(decoded);
      } catch (err) {
        return reply.code(401).send({ error: "Invalid or expired token" });
      }

      const userId = decoded.userId;
      const data = await request.file();

      if (!data) {
        return reply.code(400).send({ error: "No file provided" });
      }

      const allowedMimes = ["image/jpeg", "image/gif", "image/png"];
      if (!allowedMimes.includes(data.mimetype)) {
        return reply
          .code(400)
          .send({ error: "Only JPG, GIF, or PNG files are allowed" });
      }

      const buffer = await data.toBuffer();

      const maxSize = 1024 * 1024;
      if (buffer.length > maxSize) {
        return reply.code(400).send({ error: "File size exceeds 1MB limit" });
      }

      const fileExtension = data.mimetype.split("/")[1];
      const filename = `${userId}-${Date.now()}.${fileExtension}`;

      await UserGeneratedContent.write(filename, buffer);

      const user = await User.findByIdAndUpdate(
        userId,
        { avatar: `/${filename}` },
        { new: true }
      );

      if (!user) {
        return reply.code(404).send({ error: "User not found" });
      }

      reply.send({
        success: true,
        message: "Avatar uploaded successfully",
        avatarUrl: `/${filename}`,
      });
    } catch (err) {
      console.error("Error uploading avatar:", err);
      reply.code(500).send({ error: "Internal server error" });
    }
  });

  if (process.env.NODE_ENV !== "production") {
    app.get("/cd", async (request, reply) => {
      try {
        const user = await User.findOne({ username: request.query?.username });
        if (!user) return reply.code(404).send({ error: "No user found" });
        const domainName =
          request.body?.domain || request.query?.domain || "testdomain.com";
        if (!domainName || typeof domainName !== "string") {
          return reply.code(400).send({ error: "Domain name required" });
        }
        const testDomain = {
          domain: domainName,
          proxied: [],
          acl: [],
          rules: [],
          bannedIp: [],
          integrations: {},
        };
        let domainDoc = await Domain.findOne({ domain: testDomain.domain });
        if (!domainDoc) {
          domainDoc = await Domain.create(testDomain);
        }
        const alreadyOwned = user.domains.some(
          (d) => d.name === testDomain.domain
        );
        if (!alreadyOwned) {
          user.domains.push({
            group: "default",
            name: testDomain.domain,
            status: "active",
            lastSeen: new Date(),
          });
          await user.save();
        }
        reply.send({
          message: `Domain '${domainName}' created and added to user.`,
          user: {
            username: user.username,
            domains: user.domains,
          },
          domain: domainDoc,
        });
      } catch (err) {
        console.error(err);
        reply.code(500).send({ error: err.message });
      }
    });
  }

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

  app.get("/api/:id/:action?", async (request, reply) => {
    const { id, action } = request.params;
    if (!id) return reply.code(400).send({ error: "ID is required" });

    const user = await User.findOne({ _id: id }).lean();
    if (!user) return reply.code(404).send({ error: "User not found" });

    const safe = {
      _id: user._id,
      username: user.username,
      email: user.email,
      role: user.role,
      domains: user.domains || [],
      createdAt: user.createdAt,
      updatedAt: user.updatedAt,
      integrations: user.integrations
        ? {
            twofa: user.integrations.twofa
              ? {
                  enabled: user.integrations.twofa.enabled,
                  method: user.integrations.twofa.method,
                }
              : null,
            cloudflare: user.integrations.cloudflare
              ? { connected: true }
              : null,
            google: user.integrations.google ? { connected: true } : null,
            discord: user.integrations.discord ? { connected: true } : null,
            github: user.integrations.github ? { connected: true } : null,
            microsoft: user.integrations.microsoft ? { connected: true } : null,
          }
        : null,
    };

    if (!action) return safe;
    if (typeof user[action] === "function") {
      return reply.code(400).send({ error: "Action not allowed" });
    }
    if (safe[action] === undefined) {
      return reply.code(400).send({ error: "Invalid action" });
    }
    return { action: safe[action] };
  });

  // ClickHouse log retrieval endpoints
app.get("/api/v1/logs", async (request, reply) => {
  try {
    const range = request.query.range || "24h"
    const domain = request.query.domain || null

    const logs = await queryLogs({
      domain,
      range
    })

    const stats = domain ? await getLogStats(domain, range) : []

    return domain
      ? {
          domain,
          totalLogs: logs.length,
          range,
          stats: stats[0] || null,
          logs: logs.map((log) => ({
            timestamp: log.timestamp,
            trace_id: log.trace_id,
            method: log.method,
            host: log.host,
            path: log.path,
            ip: log.ip,
            user_agent: log.user_agent,
            referer: log.referer,
            status: log.status,
            cache: log.cache,
            duration_ms: log.duration_ms,
          })),
        }
      : { range, logs }
  } catch (err) {
    console.error("Log retrieval error:", err)
    return reply.code(500).send({ error: "Failed to retrieve logs" })
  }
})

  app.get("/api/v1/logs/stats", async (request, reply) => {
    try {
      const domain = request.query.domain || null;
      const stats = await getLogStats(domain);

      // Format stats response
      const formattedStats = stats.map((stat) => ({
        host: stat.host,
        totalRequests: stat.total,
        avgDuration: parseFloat(stat.avg_duration.toFixed(2)),
        maxDuration: parseFloat(stat.max_duration.toFixed(2)),
        statusCodes: {
          success: stat.success_2xx,
          redirect: stat.redirect_3xx,
          clientError: stat.client_error_4xx,
          serverError: stat.server_error_5xx,
        },
        successRate: (
          ((stat.success_2xx / stat.total) * 100).toFixed(2)
        ).concat("%"),
      }));

      return domain
        ? { domain, stats: formattedStats[0] || null }
        : { stats: formattedStats };
    } catch (err) {
      console.error("Stats retrieval error:", err);
      return reply.code(500).send({ error: "Failed to retrieve stats" });
    }
  });

  // Debug endpoint: List all unique domains in ClickHouse
  app.get("/api/v1/logs/domains", async (request, reply) => {
    try {
      const { executeQuery } = await import("../../utils/clickhouseClient.js");
      const result = await executeQuery(
        "SELECT DISTINCT host FROM netgoat.request_logs ORDER BY host"
      );
      return { domains: result.data?.map((r) => r.host) || [] };
    } catch (err) {
      console.error("Domains retrieval error:", err);
      return reply.code(500).send({ error: "Failed to retrieve domains" });
    }
  });

  // Debug endpoint: Total log count
  app.get("/api/v1/logs/count", async (request, reply) => {
    try {
      const { executeQuery } = await import("../../utils/clickhouseClient.js");
      const result = await executeQuery(
        "SELECT COUNT() as total FROM netgoat.request_logs"
      );
      return { total: result.data?.[0]?.total || 0 };
    } catch (err) {
      console.error("Count retrieval error:", err);
      return reply.code(500).send({ error: "Failed to retrieve count" });
    }
  });
}
