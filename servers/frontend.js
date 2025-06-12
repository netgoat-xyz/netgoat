import Fastify from "fastify";
import { Eta } from "eta";
import path from "path";
import fastifyView from "@fastify/view";
const eta = new Eta();
const app = Fastify();

app.register(fastifyView, {
  engine: {
    eta,
  },
  templates: path.join(process.cwd(), "views"),
});

app.register(require("@fastify/static"), {
  root: path.join(process.cwd(), "assets"),
  prefix: "/assets/", // optional: default '/'
});

app.get("/", async (request, reply) => {
  return reply.view("index.eta");
});

app.get("/dashboard", async (request, reply) => {
  return reply.view("dashboard/index.eta");
});

app.listen({ port: 3333 }, (err, address) => {
  if (err) {
    console.error(err);
    process.exit(1);
  }
  logger.info(`Frontend loaded at ${address}`);
});
