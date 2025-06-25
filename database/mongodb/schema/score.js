import mongoose from 'mongoose'

const ScoreSchema = new mongoose.Schema({
  ipAddress: { type: String, required: true, index: true },
  score: { type: Number, required: true },
  timestamp: { type: Date, default: Date.now }
})

export default mongoose.model('Score', ScoreSchema)
