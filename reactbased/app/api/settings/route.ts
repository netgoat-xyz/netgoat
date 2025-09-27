import { NextResponse } from "next/server";
import mongoose from "mongoose";
import NotificationSettings from "../../../../database/mongodb/schema/notificationSettings";

async function connectDB() {
  if (mongoose.connection.readyState === 0) {
    await mongoose.connect(process.env.MONGODB_URI || "mongodb://localhost:27017/netgoat");
  }
}

export async function GET() {
  try {
    await connectDB();
    const doc = await NotificationSettings.findOne();
    return NextResponse.json(doc || {});
  } catch (e) {
    return NextResponse.json({ error: "Failed to load settings" }, { status: 500 });
  }
}

export async function POST(req: Request) {
  try {
    await connectDB();
    const body = await req.json();
    let doc = await NotificationSettings.findOne();
    if (doc) {
      Object.assign(doc, body, { updatedAt: new Date() });
      await doc.save();
    } else {
      doc = new NotificationSettings({ ...body, updatedAt: new Date() });
      await doc.save();
    }
    return NextResponse.json({ ok: true });
  } catch (e) {
    return NextResponse.json({ error: "Failed to save settings" }, { status: 500 });
  }
}
