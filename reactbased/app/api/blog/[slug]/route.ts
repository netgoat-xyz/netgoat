import { NextResponse } from "next/server";
// Assuming you have a file or module for BlogPost model (e.g., in @/models/BlogPost)
// import BlogPost from "@/models/BlogPost";
import mongoose from "mongoose";

// --- MongoDB Setup (Assuming BlogPost model exists) ---

// Helper to connect to MongoDB
async function connectDB() {
  if (!mongoose.connections[0].readyState) {
    // Ensure MONGODB_URI is set in your environment variables
    await mongoose.connect(process.env.MONGODB_URI as string);
  }
}

// Mock BlogPost model for type checking and execution
// NOTE: You must replace this with your actual imported Mongoose model.
const BlogPost = {
  findOne: async (query: any) => ({
    _id: new mongoose.Types.ObjectId(),
    slug: query.slug,
    title: "Example Post",
    content: "This is a placeholder.",
  }),
  findOneAndDelete: async (query: any) => ({ message: "Deleted" }),
  findOneAndUpdate: async (query: any, updates: any, options: any) => ({
    _id: new mongoose.Types.ObjectId(),
    slug: query.slug,
    ...updates,
  }),
};

// --- Route Handlers ---

// GET /api/blog/[slug]
// Fixed: Destructuring { params } from the context argument with the required inline type definition.
export async function GET(
  req: Request,
  { params }: { params: Promise<{ slug: string }> }, // Destructured definition
) {
  try {
    await connectDB();
    const slug = params; // Accessing params directly

    // Replace with your actual model logic:
    // const post = await BlogPost.findOne({ slug }).lean();
    const post = await (BlogPost as any).findOne({ slug });

    if (!post) {
      return NextResponse.json({ error: "Not found" }, { status: 404 });
    }

    return NextResponse.json(post);
  } catch (err) {
    console.error("GET /blog/:slug error:", err);
    return NextResponse.json(
      { error: "Failed to fetch blog post" },
      { status: 500 },
    );
  }
}

// DELETE /api/blog/[slug]
// Fixed: Destructuring { params } from the context argument with the required inline type definition.
export async function DELETE(
  req: Request,
  { params }: { params: Promise<{ slug: string }> }, // Destructured definition
) {
  try {
    await connectDB();
    const slug = params; // Accessing params directly

    // Replace with your actual model logic:
    // const deleted = await BlogPost.findOneAndDelete({ slug });
    const deleted = await (BlogPost as any).findOneAndDelete({ slug });

    if (!deleted) {
      return NextResponse.json({ error: "Blog not found" }, { status: 404 });
    }

    return NextResponse.json({ message: "Blog deleted successfully" });
  } catch (err) {
    console.error("DELETE /blog/:slug error:", err);
    return NextResponse.json(
      { error: "Failed to delete blog" },
      { status: 500 },
    );
  }
}

// PATCH /api/blog/[slug]
// Fixed: Destructuring { params } from the context argument with the required inline type definition.
export async function PATCH(
  req: Request,
  { params }: { params: Promise<{ slug: string }> }, // Destructured definition
) {
  try {
    await connectDB();
    const slug = params; // Accessing params directly

    const updates = await req.json();
    if (!updates || Object.keys(updates).length === 0) {
      return NextResponse.json(
        { error: "No updates provided" },
        { status: 400 },
      );
    }

    // Replace with your actual model logic:
    // const updatedPost = await BlogPost.findOneAndUpdate({ slug }, updates, { new: true });
    const updatedPost = await (BlogPost as any).findOneAndUpdate(
      { slug },
      updates,
      { new: true },
    );

    if (!updatedPost) {
      return NextResponse.json({ error: "Blog not found" }, { status: 404 });
    }

    return NextResponse.json(updatedPost);
  } catch (err) {
    console.error("PATCH /blog/:slug error:", err);
    return NextResponse.json(
      { error: "Failed to update blog" },
      { status: 500 },
    );
  }
}
