import mongoose from "mongoose";
const { Schema } = mongoose;

const UserSchema = new Schema({
  username: {
    type: String,
    required: true,
    unique: true,
    trim: true,
  },
  password: {
    type: String,
    required: true,
    select: false, // Exclude from queries by default
  },
  email: {
    type: String,
    required: true,
    unique: true,
    trim: true,
  },
  role: {
    type: String,
    enum: ["user", "admin"],
    default: "user",
  },
  integrations: {
    cloudflare: {},
    google: {},
    discord: {},
    github: {},
    microsoft: {},
    "2fa": {
      enabled: {
        type: Boolean,
        default: false,
      },
      method: {
        type: String,
        enum: ["sms", "email", "authenticator"],
        default: "authenticator",
      },
    },
  },
  createdAt: {
    type: Date,
    default: Date.now,
  },
  updatedAt: {
    type: Date,
    default: Date.now,
  },
});

export default mongoose.model("User", UserSchema);
