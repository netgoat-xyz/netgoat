import User from "../../database/mongodb/schema/users.js";
import Domain from "../../database/mongodb/schema/domains.js";
import jsonwebtoken from "jsonwebtoken";
import Bun from "bun";
import fs from "fs";
import path from "path";

export function registerRoutes(app) {
  app.get("/", async (request, reply) => {
    return { message: "Welcome to the backend Server!" };
  });

  // Auth Register
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
        reply.code(201).send({ success: true, reply: "User created successfully" });
      })
      .catch((err) => {
        console.error("Error creating user:", err);
        reply.code(500).send({ reply: "Internal server error", success: false });
      });
              reply.code(201).send({ success: true, reply: "User created successfully" });

  });

  // Auth Login
  app.post("/api/auth/login", async (request, reply) => {
    const { username, email, password } = request.body;
    if ((!username && !email) || !password) {
      return reply.code(400).send({ error: "Username or email and password required" });
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
      console.log(jwttoken)
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

  // Update Profile
  app.post("/api/profile/update", async (request, reply) => {
    try {
      // Extract JWT from Authorization header
      const authHeader = request.headers.authorization;
      if (!authHeader || !authHeader.startsWith("Bearer ")) {
        return reply.code(401).send({ error: "Missing or invalid authorization header" });
      }
      
      const token = authHeader.slice(7); // Remove "Bearer " prefix
      let decoded;
      try {
        decoded = jsonwebtoken.verify(token, process.env.JWT_SECRET);
      } catch (err) {
        return reply.code(401).send({ error: "Invalid or expired token" });
      }

      const userId = decoded.userId;
      const { firstName, lastName, email, username, timezone } = request.body;

      // Validate input
      if (!firstName && !lastName && !email && !username && !timezone) {
        return reply.code(400).send({ error: "At least one field must be provided" });
      }

      // Check if new username is already taken (if username is being changed)
      if (username) {
        const existingUser = await User.findOne({ username, _id: { $ne: userId } });
        if (existingUser) {
          return reply.code(409).send({ error: "Username already taken" });
        }
      }

      // Check if new email is already taken (if email is being changed)
      if (email) {
        const existingUser = await User.findOne({ email, _id: { $ne: userId } });
        if (existingUser) {
          return reply.code(409).send({ error: "Email already in use" });
        }
      }

      // Update user profile
      const updateData = {};
      if (firstName) updateData.firstName = firstName;
      if (lastName) updateData.lastName = lastName;
      if (email) updateData.email = email;
      if (username) updateData.username = username;
      if (timezone) updateData.timezone = timezone;

      const user = await User.findByIdAndUpdate(userId, updateData, { new: true });
      if (!user) {
        return reply.code(404).send({ error: "User not found" });
      }

      // Return sanitized user data (no passwords or sensitive fields)
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

  // Upload Avatar
  app.post("/api/profile/avatar", async (request, reply) => {
    try {
      // Extract JWT from Authorization header
      const authHeader = request.headers.authorization;
      if (!authHeader || !authHeader.startsWith("Bearer ")) {
        return reply.code(401).send({ error: "Missing or invalid authorization header" });
      }

      const token = authHeader.slice(7); // Remove "Bearer " prefix
      let decoded;
      try {
        decoded = jsonwebtoken.verify(token, process.env.JWT_SECRET);
        console.log(decoded)
      } catch (err) {
        return reply.code(401).send({ error: "Invalid or expired token" });
      }

      const userId = decoded.userId;
      const data = await request.file();

      if (!data) {
        return reply.code(400).send({ error: "No file provided" });
      }

      // Validate file type
      const allowedMimes = ["image/jpeg", "image/gif", "image/png"];
      if (!allowedMimes.includes(data.mimetype)) {
        return reply.code(400).send({ error: "Only JPG, GIF, or PNG files are allowed" });
      }

      // Read file buffer
      const buffer = await data.toBuffer();

      // Validate file size (1MB max)
      const maxSize = 1024 * 1024;
      if (buffer.length > maxSize) {
        return reply.code(400).send({ error: "File size exceeds 1MB limit" });
      }

      // Create avatars directory if it doesn't exist
      const avatarsDir = path.join(process.cwd(), "assets", "avatars");
      if (!fs.existsSync(avatarsDir)) {
        fs.mkdirSync(avatarsDir, { recursive: true });
      }

      // Generate filename
      const fileExtension = data.mimetype.split("/")[1];
      const filename = `${userId}-${Date.now()}.${fileExtension}`;
      const filepath = path.join(avatarsDir, filename);

      // Write file to disk
      fs.writeFileSync(filepath, buffer);

      // Update user avatar field in MongoDB
      const user = await User.findByIdAndUpdate(
        userId,
        { avatar: `/avatars/${filename}` },
        { new: true }
      );

      if (!user) {
        return reply.code(404).send({ error: "User not found" });
      }

      reply.send({
        success: true,
        message: "Avatar uploaded successfully",
        avatarUrl: `/avatars/${filename}`,
      });
    } catch (err) {
      console.error("Error uploading avatar:", err);
      reply.code(500).send({ error: "Internal server error" });
    }
  });

  // Create Domain
  if (process.env.NODE_ENV !== "production") {
  app.get("/cd", async (request, reply) => {
    try {
      const user = await User.findOne({ username: request.query?.username });
      if (!user) return reply.code(404).send({ error: "No user found" });
      const domainName = request.body?.domain || request.query?.domain || "testdomain.com";
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
  }4
  // Monitor.js
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
    
    // Return sanitized user data matching User schema (no password or sensitive data)
    const safe = {
      _id: user._id,
      username: user.username,
      email: user.email,
      role: user.role,
      domains: user.domains || [],
      createdAt: user.createdAt,
      updatedAt: user.updatedAt,
      // Include non-sensitive integration info (e.g., enabled status, not tokens)
      integrations: user.integrations ? {
        twofa: user.integrations.twofa ? {
          enabled: user.integrations.twofa.enabled,
          method: user.integrations.twofa.method,
        } : null,
        // Include integration names without sensitive data
        cloudflare: user.integrations.cloudflare ? { connected: true } : null,
        google: user.integrations.google ? { connected: true } : null,
        discord: user.integrations.discord ? { connected: true } : null,
        github: user.integrations.github ? { connected: true } : null,
        microsoft: user.integrations.microsoft ? { connected: true } : null,
      } : null,
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
}
