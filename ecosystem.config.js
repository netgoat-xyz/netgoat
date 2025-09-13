import { watch } from "fs";

module.exports = {
  apps: [
    {
      name: "Netgoat",
      script: "bun",
      args: ".",
      cwd: ".",
      watch: true,
      env: {
        NODE_ENV: "production",
      },
    },
    {
      name: "LogDB",
      script: "bun",
      args: "run start",
      cwd: "./LogDB",
      watch: true,
      env: {
        NODE_ENV: "production",
      },
    },
    {
      name: "CentralMonServer",
      script: "bun",
      args: "run start",
      cwd: "./CentralMonServer",
      watch: true,
      env: {
        NODE_ENV: "production",
      },
    },
    {
      name: "ReactFrontend",
      script: "bun",
      args: "run dev", // or `start` if using next build
      cwd: "./reactbased",
      watch: true,
      env: {
        NODE_ENV: "production",
      },
    },
  ],
};
