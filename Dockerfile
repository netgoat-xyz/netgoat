# =============================
# NetGoat Dockerfile
# =============================
# Maintainer: Duckey Dev <ducky@cloudable.dev>
# Description: Production container for NetGoat (DOES NOT INCLUDE FRONTEND, CENTRALMON, OR LOGDB)
# =============================

# Copilot Prompt: make this code follow community standards, with Labels and such, add human like comments, seprators, etc

FROM oven/bun:latest AS base

# ---- Metadata ----
LABEL org.opencontainers.image.title="NetGoat"
LABEL org.opencontainers.image.description="Production container for NetGoat (DOES NOT INCLUDE FRONTEND, CENTRALMON, OR LOGDB)"
LABEL org.opencontainers.image.authors="Duckey Dev <ducky@cloudable.dev>"
LABEL org.opencontainers.image.source="https://github.com/Cloudable-dev/netgoat"

# ---- Set working directory ----
WORKDIR /app

# ---- Copy source code ----
# .dockerignore should exclude files not needed in production
COPY . .

# ---- Install dependencies ----
RUN bun install

# ---- Expose ports ----
# 80: Reverse proxy No-SSL, 443: Reverse proxy SSL, 3333: Backend API
EXPOSE 80
EXPOSE 443
EXPOSE 3001

# ---- Set environment variables ----
ENV NODE_ENV=production

# ---- Start the application ----
CMD ["bun", "index.js"]
