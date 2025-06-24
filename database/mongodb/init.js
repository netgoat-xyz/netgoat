import mongoose from "mongoose";

mongoose.connect(process.env.mongodb).then(() => {
    logger.success("Connected to MongoDB")
}).catch((err) => {
    logger.error(err.msg)
    logger.error("Crashing to prevent further damage")
    process.exit(1)
})