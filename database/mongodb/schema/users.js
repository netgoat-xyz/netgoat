import mongoose from "mongoose";
const { Schema } = mongoose;

const TwoFASchema = new Schema({
  enabled: { type: Boolean, default: false },
  method: {
    type: String,
    enum: ["sms", "email", "authenticator"],
    default: "authenticator",
  },
  authenticatorSecret: { type: String, select: false },
  phoneNumber: { type: String, select: false },
}, { _id: false });

const IntegrationsSchema = new Schema({
  cloudflare: { type: Object, default: {} },
  google: { type: Object, default: {} },
  discord: { type: Object, default: {} },
  github: { type: Object, default: {} },
  microsoft: { type: Object, default: {} },
  twofa: { type: TwoFASchema, default: () => ({}) },
}, { _id: false });

const UserSchema = new Schema({
  username: { type: String, required: true, unique: true, trim: true, index: true },
  password: { type: String, required: true },
  email: {
    type: String,
    required: true,
    unique: true,
    trim: true,
    match: [/.+@.+\..+/, "Please enter a valid email address"],
    index: true,
  },
  role: { type: String, enum: ["user", "admin"], default: "user" },
  integrations: { type: IntegrationsSchema, default: () => ({}) },
}, { timestamps: true });

export default mongoose.models.User || mongoose.model("User", UserSchema);
