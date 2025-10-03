import { NextResponse } from "next/server";
import BlogPost from "@/models/BlogPost";
import mongoose from "mongoose";

async function connectDB() {
  if (!mongoose.connections[0]?.readyState) {
    await mongoose.connect(process.env.MONGODB_URI as string);
  }
}

export async function GET() {
  try {
    await connectDB();
    const posts = await BlogPost.find().sort({ date: -1 }).lean();
    return NextResponse.json(posts);
  } catch (err: any) {
    console.error("API /blog error:", err);
    return NextResponse.json({ error: "Failed to fetch posts" }, { status: 500 });
  }
}

export async function POST(req: Request) {
  try {
    await connectDB();
    const body = await req.json();
    const { title, excerpt, category, author, readTime, content } = body;

    if (!title || !excerpt || !category || !readTime || !content) {
      return NextResponse.json({ error: "Missing required fields" }, { status: 400 });
    }

    const slug = title
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/(^-|-$)/g, "");

    const newPost = await BlogPost.create({
      title,
      excerpt,
      category,
      author,
      readTime,
      content,
      slug,
      date: new Date(),
    });

    return NextResponse.json(newPost, { status: 201 });
  } catch (err) {
    console.error(err);
    return NextResponse.json({ error: "Failed to create blog" }, { status: 500 });
  }
}

