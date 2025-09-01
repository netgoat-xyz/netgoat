

import mongoose from "mongoose";
const { Schema } = mongoose;

const ScoreSchema = new Schema({
  ipAddress: { type: String, required: true, index: true },
  score: { type: Number, required: true },
}, { timestamps: true });

export default mongoose.models.Score || mongoose.model("Score", ScoreSchema);