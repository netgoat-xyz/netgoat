import mongoose from "mongoose"
import logger from "../../utils/logger.js"

const sanitize = (uri) => {
  const match = uri.match(/mongodb(?:\+srv)?:\/\/([^:]+):([^@]+)@(.+)/)
  if (!match) return uri

  const user = match[1]
  const pass = encodeURIComponent(match[2])
  const rest = match[3]

  return `mongodb://${user}:${pass}@${rest}`
}

const raw = process.env.MONGODB_URI || ""
if (!raw) {
  logger.error("No MONGODB_URI provided")
  process.exit(1)
}

const MONGODB_URI = sanitize(raw)

let DB_READY = false

await mongoose
  .connect(MONGODB_URI, { serverSelectionTimeoutMS: 5000 })
  .then(() => {
    DB_READY = true
    logger.success(`Mongo connected`)
  })
  .catch((e) => {
    logger.error(e.message)
    logger.error("Crashing to prevent further damage")
    process.exit(1)
  })

export default DB_READY
