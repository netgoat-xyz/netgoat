import { Elysia } from 'elysia'
import { Eta } from "eta";
import path from "node:path";
import { fileURLToPath } from "url";
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const eta = new Eta({ views: path.join(__dirname, "../views") })
import staticPlugin from "@elysiajs/static";
import tracelet from '../utils/tracelet';
const app = new Elysia();

app.use(staticPlugin({
  assets: 'assets',
  prefix: "/assets",
  alwaysStatic: true,
    headers: {
    "Cache-Control": "public, max-age=31536000, immutable"
  }
}));

app.get("/", async (ctx) => {
  return reply.view("index.eta");
});

app.get("/dashboard", async (ctx) => {
  return reply.view("dashboard/index.eta");
});


// Pages for testing!!!
app.get("/error/:page", async ({ params }) => {
  const html = await eta.render(`error/${params.page}.ejs`, {
    traceletId: tracelet("MM")
  });
  return new Response(html, {
    headers: { "Content-Type": "text/html" },
  });
});

app.get("/access/:page", async ({ params }) => {
  const html = await eta.render(path.join("..", "views", "access", `${params.page}.ejs`));
  return new Response(html, {
    headers: { "Content-Type": "text/html" },
  });
});

app.listen({ port: 3333 })
  logger.info(`Frontend loaded at port 3333`);
