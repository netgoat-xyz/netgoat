import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  devIndicators: false,
  env: {
   backendapi: "http://localhost:3001",
   logdb: "http://localhost:3010",
  },
  images: {
    remotePatterns: [
      {
        protocol: "https",
        hostname: "avatars.githubusercontent.com",
        port: "",
        pathname: "/**",
      },
      {
        protocol: "https",
        hostname: "raw.githubusercontent.com",
        port: "",
        pathname: "/**",
      },
      {
        protocol: "https",
        hostname: "tapback.co",
        port: "",
        pathname: "/**",
      },
      {
        protocol: "https",
        hostname: "cdnjs.cloudflare.com",
        port: "",
        pathname: "/**"
      },
      {
        protocol: "https",
        hostname: "t2.gstatic.com",
        port: "",
        pathname: "/**"
      },
      {
        protocol: "https",
        hostname: "www.tapback.co",
        port: "",
        pathname: "/**",
      },
      {
        protocol: "https",
        hostname: "cdn.jsdelivr.net",
        port: "",
        pathname: "/**",
      },
      {
        protocol: "https",
        hostname: "cdn.discordapp.com",
        port: "",
        pathname: "/**",
      }
    ],
  }
};

export default nextConfig;
