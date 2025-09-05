"use client"

import Header from "@/components/homescreen/header"
import ShaderBackground from "@/components/homescreen/shader-background"
import BlogGrid from "@/components/homescreen/blog-grid"

export default function BlogPage() {
  return (
    <ShaderBackground>
      <Header />
      <BlogGrid />
    </ShaderBackground>
  )
}
