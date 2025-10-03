import mongoose, { Schema } from "mongoose"

const BlogPostSchema = new Schema({
  slug: { type: String, unique: true },
  title: String,
  date: String,
  readTime: String,
  excerpt: String,
  author: String,
  category: String,
  content: [String],
})

export default mongoose.models.BlogPost || mongoose.model("BlogPost", BlogPostSchema)
