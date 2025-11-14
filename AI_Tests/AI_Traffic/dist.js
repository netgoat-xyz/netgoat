import * as tf from "@tensorflow/tfjs-node";
const model = await tf.loadLayersModel("file://./ddos_tfjs_model/model.json");

const X = generateTrafficData(1000); // synthetic batch
const softLabels = model.predict(X);
fs.writeFileSync("distill_data.json", JSON.stringify({ X, softLabels }));