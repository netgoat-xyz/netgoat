"use client"

import Link from "next/link"

const blogPosts = [
  {
    slug: "advanced-shader-techniques",
    title: "Advanced Shader Techniques for Modern Web",
    excerpt:
      "Explore cutting-edge shader programming methods that push the boundaries of web graphics and create stunning visual experiences.",
    date: "Dec 15, 2024",
    readTime: "8 min read",
    category: "Technical",
  },
  {
    slug: "interactive-lighting-systems",
    title: "Building Interactive Lighting Systems",
    excerpt:
      "Learn how to create responsive lighting effects that react to user interactions and create immersive digital environments.",
    date: "Dec 12, 2024",
    readTime: "6 min read",
    category: "Tutorial",
  },
  {
    slug: "performance-optimization-shaders",
    title: "Performance Optimization for Complex Shaders",
    excerpt:
      "Master the art of optimizing shader performance while maintaining visual quality across different devices and browsers.",
    date: "Dec 10, 2024",
    readTime: "10 min read",
    category: "Performance",
  },
  {
    slug: "creative-visual-effects",
    title: "Creative Visual Effects with Paper Shaders",
    excerpt:
      "Discover innovative ways to use Paper Shaders for creating unique visual effects that captivate and engage users.",
    date: "Dec 8, 2024",
    readTime: "5 min read",
    category: "Creative",
  },
  {
    slug: "shader-fundamentals",
    title: "Shader Fundamentals: A Complete Guide",
    excerpt:
      "Start your shader journey with this comprehensive guide covering the essential concepts and techniques every developer should know.",
    date: "Dec 5, 2024",
    readTime: "12 min read",
    category: "Beginner",
  },
  {
    slug: "real-time-rendering-tips",
    title: "Real-time Rendering Tips and Tricks",
    excerpt:
      "Professional insights into real-time rendering techniques that will elevate your shader programming skills to the next level.",
    date: "Dec 3, 2024",
    readTime: "7 min read",
    category: "Advanced",
  },
]

export default function BlogGrid() {
  return (
    <main className="relative z-20 px-8 py-16">
      <div className="max-w-6xl mx-auto">
        {/* Header Section */}
        <div className="mb-16">
          <div className="inline-flex items-center px-3 py-1 rounded-full bg-white/[0.08] backdrop-blur-xl border border-white/[0.12] mb-6 relative overflow-hidden">
            <div className="absolute inset-0 bg-gradient-to-r from-white/[0.02] via-white/[0.05] to-white/[0.02]" />
            <div className="absolute top-0 left-0 right-0 h-px bg-gradient-to-r from-transparent via-white/30 to-transparent" />
            <span className="text-white/90 text-xs font-light relative z-10">📚 News Hub</span>
          </div>

          <h1 className="text-4xl md:text-5xl font-light text-white mb-4 tracking-tight">
            <span className="font-medium italic instrument">Netgoat</span> Insights
          </h1>

          <p className="text-sm font-light text-white/70 max-w-2xl leading-relaxed">
            Dive into news about development, bug fixes, tutorials, and updates here.
          </p>
        </div>

        {/* Blog Grid */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {blogPosts.map((post) => (
            <Link key={post.slug} href={`/blog/${post.slug}`}>
              <article className="group cursor-pointer aspect-square">
                <div className="p-6 rounded-3xl bg-white/[0.06] backdrop-blur-2xl border border-white/[0.08] hover:bg-white/[0.12] hover:border-white/[0.16] transition-all duration-500 ease-out h-full relative overflow-hidden transform hover:scale-[1.02] hover:-translate-y-1 flex flex-col">
                  {/* Liquid glass overlay effects */}
                  <div className="absolute inset-0 bg-gradient-to-br from-white/[0.03] via-transparent to-white/[0.02] opacity-60" />
                  <div className="absolute top-0 left-0 right-0 h-px bg-gradient-to-r from-transparent via-white/25 to-transparent" />
                  <div className="absolute bottom-0 left-0 right-0 h-px bg-gradient-to-r from-transparent via-white/10 to-transparent" />

                  {/* Subtle inner glow */}
                  <div className="absolute inset-0 rounded-3xl shadow-inner shadow-white/[0.02]" />

                  <div className="relative z-10 flex flex-col h-full">
                    {/* Category Badge */}
                    <div className="inline-flex items-center px-3 py-1.5 rounded-full bg-white/[0.12] backdrop-blur-xl border border-white/[0.08] mb-4 relative overflow-hidden w-fit">
                      <div className="absolute inset-0 bg-gradient-to-r from-white/[0.02] to-white/[0.04]" />
                      <span className="text-white/85 text-xs font-light relative z-10">{post.category}</span>
                    </div>

                    {/* Title */}
                    <h2 className="text-lg font-medium text-white mb-3 group-hover:text-white/95 transition-colors duration-300 line-clamp-2 flex-shrink-0">
                      {post.title}
                    </h2>

                    {/* Excerpt */}
                    <p className="text-xs font-light text-white/65 mb-4 leading-relaxed line-clamp-4 flex-grow">
                      {post.excerpt}
                    </p>

                    {/* Meta Info */}
                    <div className="flex items-center justify-between text-xs text-white/50 mt-auto pt-2 border-t border-white/[0.06] flex-shrink-0">
                      <span>{post.date}</span>
                      <span>{post.readTime}</span>
                    </div>
                  </div>
                </div>
              </article>
            </Link>
          ))}
        </div>
      </div>
    </main>
  )
}
