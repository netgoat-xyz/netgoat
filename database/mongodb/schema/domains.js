import mongoose from "mongoose";
const { Schema } = mongoose;

// 🔐 ACL Schema
const aclSchema = new Schema(
  {
    user: String,
    permission: {
      view: Boolean,
      edit: Boolean,
      delete: Boolean,
      access: Boolean,
      bypassRestrictions: Boolean,
    },
  },
  { _id: false }
);

// 📜 Rule Schema
const ruleSchema = new Schema(
  {
    name: String,
    priority: Boolean,
    code: String,
  },
  { _id: false }
);

// 🚫 Banned IP Schema
const bannedIPSchema = new Schema(
  {
    name: String,
    ip: String,
  },
  { _id: false }
);

// 📈 Rate Limit Rules

const rateLimitSchema = new Schema({
    requestsPerMinute: Number,
    burstLimit: Number,
});

// 📂 Violation Log

const violations = new Schema({
  ip: String,
  reason: String,
  path: String,
  time: { type: Date, default: Date.now, expires: 60 * 60 }, // TTL: 1 hour
})

// 🌐 Proxied Service Schema
const proxiedSchema = new Schema(
  {
    domain: String,
    port: Number,
    BlockCommonExploits: Boolean,
    WS: Boolean,
    ip: String,
    slug: String,
    SSL: Boolean,
    SSLInfo: {
      localCert: Boolean,
      certPaths: {
        PubKey: String,
        PrivKey: String,
      },
    },
    seperateRules: [ruleSchema],
    SeperateACL: [aclSchema],
    SeperateBannedIP: [bannedIPSchema],
      rateRules: [rateLimitSchema],
  violations: [violations],
  },
  { _id: false }
);

// 🔌 Integrations Schema
const integrationsSchema = new Schema(
  {
    CloudflareTunnels: Boolean,
    Ngrok: Boolean,
    Tailscale: Boolean,
    IntegrationDetails: {
      Cloudflare: {
        AccountID: String,
        ApiToken: String,
      },
      Ngrok: {
        ApiKey: String,
      },
    },
  },
  { _id: false }
);



// 🧩 Final Domain Schema
const domainSchema = new Schema({
  domain: { type: String, required: true, unique: true, trim: true, index: true },
  nameservers: { type: Boolean, default: false },
  proxied: [proxiedSchema],
  acl: [aclSchema],
  rules: [ruleSchema],
  bannedIp: [bannedIPSchema],
  integrations: integrationsSchema,
}, { timestamps: true });

export default mongoose.models.Domain || mongoose.model("Domain", domainSchema);