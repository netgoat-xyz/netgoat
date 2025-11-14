import * as tf from "@tensorflow/tfjs-node";

// ===== PARAMETERS =====
const DATA_SAMPLES = 1_000_000; // how many total examples
const BATCH_SIZE = 256;
const EPOCHS = 20;
const STEPS_PER_EPOCH = 5000;
const INPUT_DIM = 12;
const OUTPUT_DIM = 4;

// ===== RNG & DATA GEN =====
const RNG = (() => {
  let s = 42;
  return {
    rand() { s = (s * 1664525 + 1013904223) >>> 0; return s / 0x100000000; },
    uniform(a, b) { return a + this.rand() * (b - a); },
    randint(a, b) { return Math.floor(this.uniform(a, b + 1)); },
    choice(arr) { return arr[this.randint(0, arr.length - 1)]; },
  };
})();

const LABELS = ["Normal", "Heavy", "DoS", "DDoS"];
const LABEL_TO_INT = { Normal: 0, Heavy: 1, DoS: 2, DDoS: 3 };

function entropy(arr) {
  const c = {};
  arr.forEach(x => c[x] = (c[x] || 0) + 1);
  return Object.values(c).reduce((sum, v) => {
    const p = v / arr.length;
    return sum - p * Math.log2(p);
  }, 0);
}

function genFlow(label) {
  let pkt, bytes, duration, uniqueSrc;
  switch(label) {
    case "Normal":
      pkt = RNG.uniform(1,50); bytes=RNG.uniform(400,1500); duration=RNG.uniform(5,600); uniqueSrc=1; break;
    case "Heavy":
      pkt = RNG.uniform(200,3000); bytes=RNG.uniform(400,1500); duration=RNG.uniform(30,3600); uniqueSrc=1; break;
    case "DoS":
      pkt = RNG.uniform(3000,30000); bytes=RNG.uniform(50,400); duration=RNG.uniform(5,300); uniqueSrc=1; break;
    default: // DDoS
      pkt = RNG.uniform(1000,30000); bytes=RNG.uniform(50,700); duration=RNG.uniform(1,1200);
      uniqueSrc = RNG.randint(50,4000);
  }
  const totalPackets = Math.max(1, Math.floor(pkt * Math.max(1,duration)));
  const bytesTotal = Math.floor(totalPackets * bytes);
  const dstPort = RNG.choice([80,443,22,53,RNG.randint(1024,65535)]);
  const protocol = RNG.choice([6,17]);
  const uniquePorts = label !== "DDoS" ? RNG.randint(1,4) : RNG.randint(1,30);
  const fake = Math.floor(uniqueSrc * (label==="Normal"?0.01:label==="Heavy"?0.25:label==="DoS"?0.05:0.18));
  const impossible = fake;
  const ent = entropy(Array(uniqueSrc).fill(0));
  return [uniqueSrc,totalPackets,bytesTotal,duration,bytes,pkt,dstPort,protocol,uniquePorts,fake,impossible,ent];
}

function* dataGenerator(batchSize) {
  while(true) {
    const X = [], y = [];
    for(let i=0;i<batchSize;i++) {
      const lbl = RNG.choice(LABELS);
      X.push(genFlow(lbl));
      const onehot = Array(LABELS.length).fill(0);
      onehot[LABEL_TO_INT[lbl]] = 1;
      y.push(onehot);
    }
    yield { xs: tf.tensor2d(X), ys: tf.tensor2d(y) };
  }
}

// ===== MODEL =====
function buildModel(inputDim, outputDim) {
  const model = tf.sequential();
  model.add(tf.layers.dense({ units: 256, inputShape: [inputDim], activation: 'relu' }));
  model.add(tf.layers.dense({ units: 128, activation: 'relu' }));
  model.add(tf.layers.dropout({ rate: 0.2 }));
  model.add(tf.layers.dense({ units: 64, activation: 'relu' }));
  model.add(tf.layers.dense({ units: outputDim, activation: 'softmax' }));
  model.compile({ optimizer: tf.train.adam(0.001), loss: 'categoricalCrossentropy', metrics:['accuracy'] });
  return model;
}

// ===== TRAINING LOOP =====
async function train() {
  const model = buildModel(INPUT_DIM, OUTPUT_DIM);
  const gen = dataGenerator(BATCH_SIZE);
  await model.fitDataset(tf.data.generator(() => gen), {
    epochs: EPOCHS,
    batchesPerEpoch: STEPS_PER_EPOCH,
    verbose: 1
  });
  await model.save('file://./ai_reasoning_model');
  console.log("Saved model.");
}

train();
