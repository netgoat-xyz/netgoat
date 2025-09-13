#!/usr/bin/env bun
/**
 * NetGoat stack runner
 * Starts Core (bun .), LogDB (bun .), CentralMonServer (bun .), and Frontend (bun run dev)
 * - Does not build the frontend
 * - Streams all outputs with prefixes
 */

function colorWrap(name: string, color: string) {
  return `${color}[${name}]\x1b[0m`;
}

const COLORS = {
  core: "\x1b[36m",
  logdb: "\x1b[33m",
  ctm: "\x1b[35m",
  fe: "\x1b[32m",
};

async function pipeLines(prefix: string, stream: ReadableStream<Uint8Array>) {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buf = "";
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });
    let idx;
    while ((idx = buf.indexOf("\n")) !== -1) {
      const line = buf.slice(0, idx).replace(/\r$/, "");
      if (line.length) console.log(`${prefix} ${line}`);
      buf = buf.slice(idx + 1);
    }
  }
  if (buf.length) console.log(`${prefix} ${buf}`);
}

type ProcDef = {
  name: "core" | "logdb" | "ctm" | "fe";
  cmd: string[];
  cwd: string;
};

async function run() {
  const root = process.cwd();
  const procs: ProcDef[] = [
    { name: "core", cmd: ["bun", "."], cwd: root },
    { name: "logdb", cmd: ["bun", "."], cwd: `${root}/LogDB` },
    { name: "ctm", cmd: ["bun", "."], cwd: `${root}/CentralMonServer` },
    { name: "fe", cmd: ["bun", "run", "dev"], cwd: `${root}/reactbased` },
  ];

  const children = procs.map((p) => {
    const child = Bun.spawn({
      cmd: p.cmd,
      cwd: p.cwd,
      stdout: "pipe",
      stderr: "pipe",
      env: process.env,
    });
    const px = colorWrap(p.name, (COLORS as any)[p.name]);
    pipeLines(px, child.stdout);
    pipeLines(px, child.stderr);
    return { p, child };
  });

  const stopAll = () => {
    for (const { child, p } of children) {
      try {
        child.kill();
      } catch {}
    }
  };

  process.on("SIGINT", () => {
    stopAll();
    setTimeout(() => process.exit(0), 50);
  });

  // Wait for any to exit
  const results = await Promise.allSettled(children.map(({ child }) => child.exited));
  // If any failed, exit non-zero
  if (results.some((r) => r.status === "fulfilled" && (r as PromiseFulfilledResult<number>).value !== 0)) {
    process.exit(1);
  }
}

run().catch((err) => {
  console.error("[stack] error:", err);
  process.exit(1);
});
