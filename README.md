<img width="5658" height="1600" alt="image" src="https://github.com/user-attachments/assets/d30fb971-4b39-490c-ac08-0d688e8f9ada" />

# NetGoat — Self-Hostable Cloudflare Alternative (Reverse Proxy Engine)

## 💖 Special Thanks

A huge thank you to **Cozy Critters Society** and **Snow** for being our first donors! Their support means the world to us. Check out their nonprofit here: [Cozy Critters Society](https://opencollective.com/cozy-critters-society).
A huge thank you to **furstxd** for contributing with the project with a Pull Request!

> _“The team at Cozy Critters Society is happy to support the development of NetGoat in hopes that we can help them succeed in making their self-hostable Cloudflare alternative.”_

---

## TLDR: Work In Progess

Hii! Its ducky the project is Work In Progress and will be publicly working beta at December

[![Bun](https://img.shields.io/badge/Bun-000?logo=bun&logoColor=fff)](https://github.com/netgoat-xyz/netgoat)

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

[![CSS](https://img.shields.io/badge/CSS-639?logo=css&logoColor=fff)](https://github.com/netgoat-xyz/netgoat) [![HTML](https://img.shields.io/badge/HTML-%23E34F26.svg?logo=html5&logoColor=white)](https://github.com/netgoat-xyz/netgoat) [![JavaScript](https://img.shields.io/badge/JavaScript-F7DF1E?logo=javascript&logoColor=000)](https://github.com/netgoat-xyz/netgoat) [![Git](https://img.shields.io/badge/Git-F05032?logo=git&logoColor=fff)](https://github.com/netgoat-xyz/netgoat) [![TypeScript](https://img.shields.io/badge/TypeScript-3178C6?logo=typescript&logoColor=fff)](https://github.com/netgoat-xyz/netgoat) [![Bash](https://img.shields.io/badge/Bash-4EAA25?logo=gnubash&logoColor=fff)](https://github.com/netgoat-xyz/netgoat) [![Cloudflare](https://img.shields.io/badge/Cloudflare-F38020?logo=Cloudflare&logoColor=white)](https://github.com/netgoat-xyz/netgoat) [![Node.js](https://img.shields.io/badge/Node.js-6DA55F?logo=node.js&logoColor=white)](https://github.com/netgoat-xyz/netgoat) [![Next.js](https://img.shields.io/badge/Next.js-black?logo=next.js&logoColor=white)](https://github.com/netgoat-xyz/netgoat) [![shadcn/ui](https://img.shields.io/badge/shadcn%2Fui-000?logo=shadcnui&logoColor=fff)](https://github.com/netgoat-xyz/netgoat) [![Tailwind CSS](https://img.shields.io/badge/Tailwind%20CSS-%2338B2AC.svg?logo=tailwind-css&logoColor=white)](https://github.com/netgoat-xyz/netgoat) [![SQLite](https://img.shields.io/badge/SQLite-%2307405e.svg?logo=sqlite&logoColor=white)](https://github.com/netgoat-xyz/netgoat) [![MongoDB](https://img.shields.io/badge/MongoDB-%234ea94b.svg?logo=mongodb&logoColor=white)](https://github.com/netgoat-xyz/netgoat)
[![Docker](https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=fff)](https://github.com/netgoat-xyz/netgoat)
[![Python](https://img.shields.io/badge/Python-3776AB?logo=python&logoColor=fff)](https://github.com/netgoat-xyz/netgoat)
[![TensorFlow](https://img.shields.io/badge/TensorFlow-ff8f00?logo=tensorflow&logoColor=white)](https://github.com/netgoat-xyz/netgoat)

![Stats](https://hackatime-badge.hackclub.com/U082B71HP8B/NetGoat)

> Built for [HackClub Summer of Making](https://summer.hackclub.com)

> Join our discord for support, annoucements, updates & bugs!! [Click Me To Join!](https://discord.com/invite/3aJ7MdJsZV) ![Discord](https://img.shields.io/discord/1350110102337749062)

NetGoat is an advanced reverse proxy engine designed to act as an **additional layer** on top of Cloudflare — enabling **premium-grade features**, **zero-cost scaling**, and **maximum control** for power users and homelabbers.

---

## Screenshots

Say cheese!
<img width="1639" height="1114" alt="image" src="https://github.com/user-attachments/assets/10590637-07b6-48c5-b083-1c13c69b9a67" />
<img width="1636" height="1131" alt="image" src="https://github.com/user-attachments/assets/36381a53-b201-4961-ab39-3f583033d75a" />
<img width="1649" height="1109" alt="image" src="https://github.com/user-attachments/assets/e5890bf2-769a-4487-8442-6a0ab0e17d3d" />
<img width="1630" height="1120" alt="image" src="https://github.com/user-attachments/assets/a294d0c0-019e-4cac-904e-6f5a10b33b6a" />

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

## Quick Start

We recommend [datalix](https://datalix.eu/a/netgoat) for cheap and highly avaliable vps'ses

https://docs.netgoat.xyz (not published yet)

## Running Services with systemd (Linux)

Prefer systemd over PM2? You can automate unit creation with the included script.

Automated one-liner (installs units for core, LogDB, CTM and Frontend):

Note: requires Bun installed and root privileges.

curl -fsSL https://raw.githubusercontent.com/cloudable-dev/NetGoat/main/scripts/install-systemd.sh | sudo bash -s -- --root-dir /opt/netgoat

Or run locally from the repo:

sudo bash scripts/install-systemd.sh --root-dir "$(pwd)" --build-frontend

Useful flags:

- --user <user> / --group <group>: system user/group to run services (default: netgoat)
- --no-netgoat, --no-logdb, --no-ctm, --no-frontend: skip specific services
- --include-docs: also install the docs site service from ./docs
- --dev-frontend / --dev-docs: run Next.js in dev mode instead of prod
- --build-frontend / --build-docs: run bun run build before creating units
- --no-start: write units but do not enable/start them

Services created:

- netgoat.service (root)
- netgoat-logdb.service (./LogDB)
- netgoat-ctm.service (./CentralMonServer)
- netgoat-frontend.service (./reactbased)
- netgoat-docs.service (./docs, optional)

Ports to allow (typical): 80, 443, 1933, 3000, 3010, 2222.

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
