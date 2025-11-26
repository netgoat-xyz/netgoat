import mongoose from "mongoose";

function sanitizeMongoURI(uri) {
    try {
        const url = new URL(uri);
        if (url.password) {
            url.password = encodeURIComponent(url.password);
        }

        return url.toString();
    } catch (err) {
        console.error("Invalid MongoDB URI:", err);
        return uri; 
    }
}

const safeURI = sanitizeMongoURI(process.env.mongodb);

mongoose.connect(safeURI).then(() => {
    logger.success("Connected to MongoDB");
}).catch((err) => {
    logger.error(err);
    logger.error("Crashing to prevent further damage");
    process.exit(1);
});
