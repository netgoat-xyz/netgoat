# =============================
# NetGoat Dockerfile
# =============================
# Maintainer: Duckey Dev <ducky@cloudable.dev>
# Description: Production container for NetGoat Frontend (DOES NOT INCLUDE Main_Files, LogDB , OR Frontend)
# =============================

# Copilot Prompt: make this code follow community standards, with Labels and such, add human like comments, seprators, etc

FROM bun:latest AS base

# ---- Metadata ----
LABEL org.opencontainers.image.title="NetGoat Frontend"
LABEL org.opencontainers.image.description="Production container for NetGoat (DOES NOT INCLUDE Main_Files, LogDB , OR Frontend)"
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
# 1933: CentralMon
EXPOSE 1933

# ---- Set environment variables ----
ENV NODE_ENV=production

# ---- Start the application ----
CMD ["bun", "."]