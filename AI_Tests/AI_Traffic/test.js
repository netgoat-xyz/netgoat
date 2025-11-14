import * as tf from "@tensorflow/tfjs-node";
import readline from "readline";

const MODEL_PATH = process.env.MODEL_PATH || "file://./ai_reasoning_model/model.json";
const INPUT_DIM = 12;
const OUTPUT_DIM = 4;
let running = false;
let intervalHandle = null;
let intervalMs = 500;


const RNG = (() => {
  let s = 42;
  return {
    rand() { s = (s * 1664525 + 1013904223) >>> 0; return s / 0x100000000; },
    uniform(a, b) { return a + this.rand() * (b - a); },
    randint(a, b) { return Math.floor(this.uniform(a, b + 1)); },
    choice(arr) { return arr[this.randint(0, arr.length - 1)]; },
  };
})();

function entropy(arr) {
  const c = {};
  arr.forEach(x => c[x] = (c[x] || 0) + 1);
  return Object.values(c).reduce((sum, v) => { const p = v / arr.length; return sum - p * Math.log2(p); }, 0);
}

function genFlowSample() {
  const label = RNG.choice(["Normal","Heavy","DoS","DDoS"]);
  let pkt, bytes, duration, uniqueSrc;
  switch(label) {
    case "Normal": pkt=RNG.uniform(1,50); bytes=RNG.uniform(400,1500); duration=RNG.uniform(5,600); uniqueSrc=1; break;
    case "Heavy": pkt=RNG.uniform(200,3000); bytes=RNG.uniform(400,1500); duration=RNG.uniform(30,3600); uniqueSrc=1; break;
    case "DoS": pkt=RNG.uniform(3000,30000); bytes=RNG.uniform(50,400); duration=RNG.uniform(5,300); uniqueSrc=1; break;
    default: pkt=RNG.uniform(1000,30000); bytes=RNG.uniform(50,700); duration=RNG.uniform(1,1200); uniqueSrc=RNG.randint(50,4000);
  }
  const totalPackets = Math.max(1, Math.floor(pkt*duration));
  const bytesTotal = Math.floor(totalPackets*bytes);
  const dstPort = RNG.choice([80,443,22,53,RNG.randint(1024,65535)]);
  const protocol = RNG.choice([6,17]);
  const uniquePorts = RNG.randint(1, label==="DDoS"?30:4);
  const fake = Math.floor(uniqueSrc*0.1);
  const impossible = fake;
  const ent = entropy(Array(uniqueSrc).fill(0));
  return [uniqueSrc,totalPackets,bytesTotal,duration,bytes,pkt,dstPort,protocol,uniquePorts,fake,impossible,ent];
}

async function main() {
  const model = await tf.loadLayersModel(MODEL_PATH);
  console.log("Model loaded from", MODEL_PATH);

  const rl = readline.createInterface({ input: process.stdin, output: process.stdout, prompt: "cmd> " });
  let running = false, intervalMs = 500;

  async function emitOnce() {
    const sample = genFlowSample();
    const X = tf.tensor2d([sample]);
    const pred = model.predict(X).arraySync()[0];
    const maxIdx = pred.indexOf(Math.max(...pred));
    console.log(`${new Date().toISOString()} | Pred=${maxIdx} (${pred.map(p=>p.toFixed(2))})`);
  }

  function startStream() {
    if(running) return console.log("Already running");
    running = true;
    intervalHandle = setInterval(()=>emitOnce().catch(console.error), intervalMs);
    console.log("Stream started");
  }

  function stopStream() {
    if(!running) return console.log("Not running");
    running = false;
    clearInterval(intervalHandle);
    console.log("Stream stopped");
  }

  rl.on("line", async (line)=>{
    const parts = line.trim().split(/\s+/);
    const cmd = parts[0];
    if(cmd==="exit"||cmd==="quit"){ stopStream(); rl.close(); return; }
    if(cmd==="run"){ await emitOnce(); rl.prompt(); return; }
    if(cmd==="start"){ startStream(); rl.prompt(); return; }
    if(cmd==="stop"){ stopStream(); rl.prompt(); return; }
    if(cmd==="interval"){ const v=Number(parts[1]); if(v>=10){ intervalMs=v; console.log("Interval set to",v,"ms"); if(running){ stopStream(); startStream(); } } rl.prompt(); return; }
    console.log("Unknown command"); rl.prompt();
  });

  rl.prompt();
}

main().catch(e=>{ console.error(e); process.exit(1); });
