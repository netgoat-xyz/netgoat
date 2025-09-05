"use client"

import Link from "next/link"

interface BlogPostProps {
  slug: string
}

const blogContent: Record<
  string,
  {
    title: string
    date: string
    readTime: string
    category: string
    content: string[]
  }
> = {
  "advanced-shader-techniques": {
    title: "Advanced Shader Techniques for Modern Web",
    date: "Dec 15, 2024",
    readTime: "8 min read",
    category: "Technical",
    content: [
      "Modern web development has evolved to embrace sophisticated visual experiences that were once exclusive to desktop applications. Shader programming stands at the forefront of this revolution, enabling developers to create stunning graphics directly in the browser.",
      "Advanced shader techniques involve understanding the GPU pipeline, optimizing vertex and fragment shaders, and leveraging modern WebGL capabilities. These techniques allow for real-time rendering of complex visual effects that respond dynamically to user interactions.",
      "One of the most powerful aspects of advanced shader programming is the ability to create procedural animations and effects. By manipulating vertices in real-time and applying complex mathematical functions to fragment colors, developers can achieve effects that would be impossible with traditional CSS animations.",
      "Performance considerations are crucial when implementing advanced shader techniques. Understanding GPU memory management, texture optimization, and draw call reduction can mean the difference between a smooth 60fps experience and a stuttering application.",
      "The future of web graphics lies in the seamless integration of shader programming with modern web frameworks. As WebGPU becomes more widely adopted, we can expect even more sophisticated shader techniques to become accessible to web developers.",
    ],
  },
  "interactive-lighting-systems": {
    title: "Building Interactive Lighting Systems",
    date: "Dec 12, 2024",
    readTime: "6 min read",
    category: "Tutorial",
    content: [
      "Interactive lighting systems transform static web experiences into dynamic, engaging environments that respond to user behavior. These systems create a sense of depth and realism that draws users into the digital space.",
      "The foundation of interactive lighting lies in understanding light physics and how to simulate them efficiently in real-time. This includes concepts like ambient, diffuse, and specular lighting, as well as shadow casting and light attenuation.",
      "Implementation begins with setting up a basic lighting model using shaders. The Phong or Blinn-Phong lighting models provide excellent starting points for creating realistic lighting effects that can be computed efficiently on the GPU.",
      "Adding interactivity requires careful consideration of performance. Mouse tracking, touch events, and device orientation can all influence lighting parameters, but these updates must be optimized to maintain smooth frame rates.",
      "Advanced interactive lighting systems can incorporate multiple light sources, dynamic shadows, and even global illumination effects. These create truly immersive experiences that blur the line between web applications and high-end graphics applications.",
    ],
  },
}

export default function BlogPost({ slug }: BlogPostProps) {
  const post = blogContent[slug]

  if (!post) {
    return (
      <main className="relative z-20 px-8 py-16">
        <div className="max-w-4xl mx-auto text-center">
          <h1 className="text-3xl font-light text-white mb-4">Post Not Found</h1>
          <p className="text-white/70 mb-8">The blog post you're looking for doesn't exist.</p>
          <Link
            href="/blog"
            className="inline-flex items-center px-6 py-3 rounded-full bg-white text-black font-normal text-sm hover:bg-white/90 transition-all duration-200"
          >
            ← Back to Blog
          </Link>
        </div>
      </main>
    )
  }

  return (
    <main className="relative z-20 px-8 py-16">
      <div className="max-w-4xl mx-auto">
        {/* Back Button */}
        <Link
          href="/blog"
          className="inline-flex items-center text-white/70 hover:text-white text-sm font-light mb-8 transition-colors duration-200"
        >
          <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 19l-7-7 7-7" />
          </svg>
          Back to Blog
        </Link>

        {/* Article Header */}
        <header className="mb-12">
          <div
            className="inline-flex items-center px-3 py-1 rounded-full bg-white/5 backdrop-blur-sm mb-6 relative"
            style={{ filter: "url(#glass-effect)" }}
          >
            <div className="absolute top-0 left-1 right-1 h-px bg-gradient-to-r from-transparent via-white/20 to-transparent rounded-full" />
            <span className="text-white/90 text-xs font-light relative z-10">{post.category}</span>
          </div>

          <h1 className="text-4xl md:text-5xl font-light text-white mb-6 tracking-tight leading-tight">{post.title}</h1>

          <div className="flex items-center gap-4 text-sm text-white/60">
            <span>{post.date}</span>
            <span>•</span>
            <span>{post.readTime}</span>
          </div>
        </header>

        {/* Article Content */}
        <article className="prose prose-invert max-w-none">
          <div className="space-y-6">
            {post.content.map((paragraph, index) => (
              <p key={index} className="text-white/80 leading-relaxed text-base font-light">
                {paragraph}
              </p>
            ))}
          </div>
        </article>

        {/* Footer */}
        <footer className="mt-16 pt-8 border-t border-white/10">
          <div className="flex items-center justify-between">
            <Link
              href="/blog"
              className="inline-flex items-center px-6 py-3 rounded-full bg-transparent border border-white/30 text-white font-normal text-sm hover:bg-white/10 hover:border-white/50 transition-all duration-200"
            >
              ← More Articles
            </Link>

            <div className="text-white/50 text-sm">Share this article</div>
          </div>
        </footer>
      </div>
    </main>
  )
}
