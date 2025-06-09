# 🐐 NetGoat

> A ruthless reverse proxy. Think Cloudflare — but angrier, open source, and way more capable.

NetGoat is an advanced reverse proxy engine designed to act as an **additional layer** on top of Cloudflare — enabling **premium-grade features**, **zero-cost scaling**, and **maximum control** for power users and homelabbers.

**⚠️ Use responsibly. This tool gives you god-mode over your web traffic.**

---

## 🚀 Features

- 🛡️ **Anti-DDoS & WAF** — Filters like a hawk. Blocks malicious requests, bots, and common exploits.
- ⚡ **Rate Limiting & Request Queuing** — Your API won’t get nuked.
- 🔒 **Auto SSL & TLS Termination** — Free SSL with auto-renew.
- 🔁 **Load Balancing & Failover** — Multinode routing with zero-downtime.
- 🔥 **Real-Time Metrics Dashboard** — Monitor traffic, bandwidth, errors, and hits.
- 🧠 **Dynamic Rules Engine** — Write custom rules in JS/TS to handle routing, caching, filtering, etc.
- 🌀 **WebSocket & HTTP/2 Ready** — Handles modern protocols like a beast.
- 🧱 **No External DB Needed** — Fully portable, flatfile configs, optional JSON-based dynamic backend.
- 🗂️ **Per-Domain Configs** — Define behavior per site with regex/wildcard support.
- 🧬 **Plugin System** — Extend NetGoat with custom plugins or middlewares.
- 🔗 **Cloudflare Zero Trust Support** — Acts as a trusted upstream in Zero Trust setups.
- 🧠 **Smart Caching Layer** — Custom cache policies per route, endpoint, or asset.

## 🔌 Seamless intergration

- 🧭 **DNS Searching** — Automatically scans your domains to automatically create a suitable Proxy record
- ☁️ **Cloudflare** — Manage cloudflare tunnels and more with our UI
- 📏 **Bandwidth Limits** — Limit or throttle specific domains or proxy's

## 🐳 Quick Start
You’ll need:
- Node or Bun (we love Bun ❤️)
- A VPS or server behind Cloudflare
- Ports 80/443 open
- Basic knowledge of how not to break the internet