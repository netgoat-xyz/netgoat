import mongoose from "mongoose";
const { Schema } = mongoose;

const rateLimitHitSchema = new Schema({
      ip: String,
  slug: String,
  ts: { type: Date, default: Date.now, expires: 60 },
})

export default mongoose.model("RateHit", rateLimitHitSchema);
