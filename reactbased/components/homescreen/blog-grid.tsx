"use client";

import { useEffect, useState } from "react";
import Link from "next/link";

interface BlogPost {
  slug: string;
  title: string;
  excerpt: string;
  date: string;
  readTime: string;
  category: string;
}

export default function BlogGrid() {
  const [posts, setPosts] = useState<BlogPost[]>([]);
  const [message, setMessage] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function fetchPosts() {
      try {
        const res = await fetch("/api/blog");
        const data = await res.json();

        if (Array.isArray(data)) {
          setPosts(data);
        } else {
          setPosts(data.posts || []);
          setMessage(data.message || null);
        }
      } catch (err) {
        setMessage("Oops! Either database went kaboom or no posts yet :(");
      } finally {
        setLoading(false);
      }
    }
    fetchPosts();
  }, []);

  if (loading) {
    return (
      <main className="px-8 py-16 text-center text-white/70">Loading…</main>
    );
  }

  return (
    <main className="relative z-20 px-8 py-16">
      <div className="max-w-6xl mx-auto">
        {/* Header Section */}
        <div className="mb-16">
          <div className="inline-flex items-center px-3 py-1 rounded-full bg-white/[0.08] backdrop-blur-xl border border-white/[0.12] mb-6 relative overflow-hidden">
            <div className="absolute inset-0 bg-gradient-to-r from-white/[0.02] via-white/[0.05] to-white/[0.02]" />
            <div className="absolute top-0 left-0 right-0 h-px bg-gradient-to-r from-transparent via-white/30 to-transparent" />
            <span className="text-white/90 text-xs font-light relative z-10">
              📚 News Hub
            </span>
          </div>

          <h1 className="text-4xl md:text-5xl font-light text-white mb-4 tracking-tight">
            <span className="font-medium italic instrument">Netgoat</span>{" "}
            Insights
          </h1>

          <p className="text-sm font-light text-white/70 max-w-2xl leading-relaxed">
            Dive into news about development, bug fixes, tutorials, and updates
            here.
          </p>
        </div>

        {/* Blog Grid */}
        {posts.length === 0 ? (
          <div className="flex flex-col items-center justify-center text-center py-20">
            <h2 className="text-2xl font-semibold text-shadow tracking-ide text-foreground/65 mb-4">
              Either no post&apos;s yet or database went kaboom
            </h2>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {posts.map((post) => (
              <Link key={post.slug} href={`/blog/${post.slug}`}>
                <article className="group cursor-pointer aspect-square">
                  <div className="p-6 rounded-3xl bg-white/[0.06] backdrop-blur-2xl border border-white/[0.08] hover:bg-white/[0.12] hover:border-white/[0.16] transition-all duration-500 ease-out h-full relative overflow-hidden transform hover:scale-[1.02] hover:-translate-y-1 flex flex-col">
                    <div className="absolute inset-0 bg-gradient-to-br from-white/[0.03] via-transparent to-white/[0.02] opacity-60" />
                    <div className="absolute top-0 left-0 right-0 h-px bg-gradient-to-r from-transparent via-white/25 to-transparent" />
                    <div className="absolute bottom-0 left-0 right-0 h-px bg-gradient-to-r from-transparent via-white/10 to-transparent" />
                    <div className="absolute inset-0 rounded-3xl shadow-inner shadow-white/[0.02]" />

                    <div className="relative z-10 flex flex-col h-full">
                      <div className="inline-flex items-center px-3 py-1.5 rounded-full bg-white/[0.12] backdrop-blur-xl border border-white/[0.08] mb-4 relative overflow-hidden w-fit">
                        <div className="absolute inset-0 bg-gradient-to-r from-white/[0.02] to-white/[0.04]" />
                        <span className="text-white/85 text-xs font-light relative z-10">
                          {post.category}
                        </span>
                      </div>

                      <h2 className="text-lg font-medium text-white mb-3 group-hover:text-white/95 transition-colors duration-300 line-clamp-2 flex-shrink-0">
                        {post.title}
                      </h2>

                      <p className="text-xs font-light text-white/65 mb-4 leading-relaxed line-clamp-4 flex-grow">
                        {post.excerpt}
                      </p>

                      <div className="flex items-center justify-between text-xs text-white/50 mt-auto pt-2 border-t w-full border-white/[0.06] flex-shrink-0">
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
                        <span>{post.readTime}</span>
                      </div>
                    </div>
                  </div>
                </article>
              </Link>
            ))}
          </div>
        )}
      </div>
    </main>
  );
}
