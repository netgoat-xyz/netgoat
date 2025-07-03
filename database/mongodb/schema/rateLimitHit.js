import mongoose from "mongoose";
const { Schema } = mongoose;

const RateLimitHitSchema = new Schema({
  ip: { type: String, required: true, index: true },
  slug: { type: String, required: true },
  ts: { type: Date, default: Date.now, expires: 60 },
}, { timestamps: true });

export default mongoose.models.RateLimitHit || mongoose.model("RateLimitHit", RateLimitHitSchema);
