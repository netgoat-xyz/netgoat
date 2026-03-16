<img width="5658" height="1600" alt="image" src="https://github.com/user-attachments/assets/d30fb971-4b39-490c-ac08-0d688e8f9ada" />

# NetGoat - Self-Hostable Cloudflare Alternative (Reverse Proxy Engine)

> [!IMPORTANT]
> NetGoat is currently in Active Alpha. We are refining the core proxy engine and self-hosting scripts for a stable public release. Follow our progress on [Discord.](https://discord.com/invite/3aJ7MdJsZV)

## 💖 Special Thanks

A huge thank you to **Cozy Critters Society** and **Snow** for being our first donors! Their support means the world to us. Check out their nonprofit here: [Cozy Critters Society](https://opencollective.com/cozy-critters-society).

> _“The team at Cozy Critters Society is happy to support the development of NetGoat in hopes that we can help them succeed in making their self-hostable Cloudflare alternative.”_

---

**NetGoat** is a **blazing-fast, self-hostable reverse proxy and traffic manager** designed for developers, homelabbers, and teams who want **Cloudflare-like features** without the cost.

Key Features:

- **Zero Trust Networking** – secure your services without hassle.
- **DDoS Protection** – keep your traffic safe from attacks.
- **SSL Termination** – handle certificates automatically.
- **Rate Limiting** – control traffic and prevent abuse.
- **WebSocket Support** – real-time apps? No problem.

Built with **modern tools** for maximum performance and developer experience:

- **Bun** for super-fast runtime.
- **Next.js** for robust front-end.
- **Fastify** for high-performance backend.
- **TailwindCSS** for sleek, responsive UI.

**NetGoat** gives you full control over your traffic, security, and performance—**all self-hosted**.

### 🛠 The Stack
- **Engine:** [Go](https://go.dev/) (High-concurrency proxying)
- **Runtime:** [Bun](https://bun.sh/) (Fast scripting & automation)
- **Control Plane:** [Next.js 15](https://nextjs.org/) & [Shadcn/ui](https://ui.shadcn.com/)
- **Storage:** [SQLite](https://sqlite.org/) (Local-first) & [MongoDB](https://www.mongodb.com/) 


> Join our discord for support, annoucements, updates & bugs!! [Click Me To Join!](https://discord.com/invite/3aJ7MdJsZV) ![Discord](https://img.shields.io/discord/1350110102337749062)

NetGoat is an advanced reverse proxy engine designed to act as an **additional layer** on top of Cloudflare — enabling **premium-grade features**, **zero-cost scaling**, and **maximum control** for power users and homelabbers.

---

## Screenshots
| Dashboard | Domain Home |
| :---: | :---: |
| <img src="https://cdn.hackclub.com/019cf11c-0d2d-7358-9037-9f91d43454b4/Screenshot%202026-03-15%20at%2017-16-53%20Me%20Team%20Dashboard.png" width="1670" height="1060" /> | <img src="https://cdn.hackclub.com/019cf11b-ec86-78e7-bca9-a662abbce384/Screenshot%202026-03-15%20at%2017-17-08%20NetGoat.png" width="100%"  height="100%" /> |

## Features

- **Anti-DDoS & WAF** — Filters like a hawk. Blocks malicious requests, bots, and common exploits.
- **Rate Limiting & Request Queuing** — Your API won’t get nuked.
- **Auto SSL & TLS Termination** — Free SSL with auto-renew.
- **Load Balancing & Failover** — Multinode routing with zero-downtime.
- **Real-Time Metrics Dashboard** — Monitor traffic, bandwidth, errors, and hits.
- **Dynamic Rules Engine** — Write custom rules in JS/TS to handle routing, caching, filtering, etc.
- **WebSocket & HTTP/2 Ready** — Handles modern protocols like a beast.
- **Per-Domain Configs** — Define behavior per site with regex/wildcard support.
- **Plugin System** — Extend NetGoat with custom plugins or middlewares.
- **Cloudflare Zero Trust Support** — Acts as a trusted upstream in Zero Trust setups.
- **Smart Caching Layer** — Custom cache policies per route, endpoint, or asset.

## Seamless intergration

- **DNS Searching** — Automatically scans your domains to automatically create a suitable Proxy record
- **Cloudflare** — Manage cloudflare tunnels and more with our UI
- **Bandwidth Limits** — Limit or throttle specific domains or proxy's

## 🏢 For Enterprise & Sponsors
NetGoat is building the future of open-source edge networking. We are looking for infrastructure partners (Bare Metal, VPS providers, Security firms) to help us battle-test the engine.

- **Strategic Partnerships:** Reach out via [Discord](https://discord.com/invite/3aJ7MdJsZV) or [Gmail](mailto:duckeydev@gmail.com)
- **Financial Support:** [GitHub Sponsors](#) | [Open Collective](...)

## Quick Start

We recommend [datalix](https://datalix.eu/a/netgoat) for cheap and highly avaliable vps'ses

https://docs.netgoat.xyz (not published yet)

## Open Source Projects That Helped me Build

- [Bun](https://bun.sh) - [Github](https://github.com/oven-sh/bun) - MIT License

- [ShadCN](https://ui.shadcn.com) - [Github](https://github.com/shadcn-ui/ui) - MIT License

- [NextJS](https://nextjs.org/) - [Github](https://github.com/vercel/next.js/) - MIT License

- [Fastify](https://fastify.dev) - [Github](https://github.com/fastify/fastify) - MIT License

- [TailwindCSS](https://tailwindcss.com) - [Github](https://github.com/tailwindlabs/tailwindcss) - MIT License

## Star History

<a href="https://www.star-history.com/#cloudable-dev/netgoat&Date">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=cloudable-dev/netgoat&type=Date&theme=dark" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=cloudable-dev/netgoat&type=Date" />
   <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=cloudable-dev/netgoat&type=Date" />
 </picture>
</a>
