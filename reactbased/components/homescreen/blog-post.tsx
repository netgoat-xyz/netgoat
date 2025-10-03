"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";

interface BlogData {
  title: string;
  subtitle?: string;
  date: string;
  readTime: string;
  category: string;
  content: string;
  author: string;
  featuredImage?: string;
}

interface BlogPostProps {
  slug: string;
}

export default function BlogPost({ slug }: BlogPostProps) {
  const [post, setPost] = useState<BlogData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function fetchPost() {
      try {
        const res = await fetch(`/api/blog/${slug}`);
        if (!res.ok) throw new Error("Not found");
        const data = await res.json();
        setPost(data);
      } catch {
        setPost(null);
      } finally {
        setLoading(false);
      }
    }
    fetchPost();
  }, [slug]);

  if (loading)
    return (
      <main className="px-8 py-16 text-center text-white/70">Loading…</main>
    );
  if (!post)
    return (
      <main className="px-8 py-16 text-center text-white/70">
        <h1 className="text-3xl font-light mb-4">Post Not Found</h1>
        <Link href="/blog" className="underline">
          Back to Blog
        </Link>
      </main>
    );

  return (
    <main className="relative z-20 px-8 py-16">
      <div className="max-w-4xl mx-auto">
        <Link
          href="/blog"
          className="text-white/70 hover:text-white text-sm font-light mb-8 inline-flex items-center gap-2"
        >
          ← Back to Blog
        </Link>

        {/* Featured Image */}
        {post.featuredImage && (
          <div
            className="h-64 w-full bg-cover bg-center rounded-xl mb-6"
            style={{ backgroundImage: `url(${post.featuredImage})` }}
          />
        )}

        {/* Title */}
        <h1 className="text-4xl md:text-5xl font-light text-white mb-4 leading-tight">
          {post.title}
        </h1>
        {post.subtitle && <p className="text-white/70 mb-6">{post.subtitle}</p>}
        {/* Category */}

        {/* Meta */}
<div className="flex gap-4 items-center text-sm text-white/60 mb-12">
  <span>
    {post.date
      ? new Date(post.date).toLocaleDateString("en-US", {
          year: "numeric",
          month: "long",
          day: "numeric",
        })
      : new Date(0).toLocaleDateString("en-US", {
          year: "numeric",
          month: "long",
          day: "numeric",
        })}
  </span>
  • <span>{post.readTime}</span>
  <div className="bg-white/25 h-6 w-[1px]"></div>

  {/* Avatar + Author container */}
  <div className="flex items-center gap-2">
    <Avatar className="h-6 w-6">
      <AvatarImage src={`https://www.tapback.co/api/avatar/${post.author}.webp`} />
      <AvatarFallback>
        {post.author
          ? post.author
              .trim()
              .split(/\s+/)
              .slice(0, 2)
              .map(w => w[0].toUpperCase())
              .join("")
          : "UA"}
      </AvatarFallback>
    </Avatar>
    <span>{post.author || "Unknown Author"}</span>
  </div>

  <div className="bg-white/25 h-6 w-[1px]"></div>
  <span className="px-3 py-1 rounded-full bg-white/5 text-xs text-white/80">
    {post.category}
  </span>
</div>
        {/* Markdown content */}
        <article className="prose prose-invert max-w-none space-y-6">
          <ReactMarkdown remarkPlugins={[remarkGfm]}>
            {post.content[0]}
          </ReactMarkdown>
        </article>

        <footer className="mt-16 pt-8 border-t border-white/10 flex justify-between items-center">
          <Link href="/blog" className="text-white/50 hover:text-white text-sm">
            ← More Articles
          </Link>
          <div className="text-white/50 text-sm">Share this article</div>
        </footer>
      </div>
    </main>
  );
}
