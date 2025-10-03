import Header from "@/components/homescreen/header"
import HeroContent from "@/components/homescreen/hero-content"
import ShaderBackground from "@/components/homescreen/shader-background"
import BlogPost from "@/components/homescreen/blog-post"


interface BlogPostPageProps {
  params: Promise<{ slug: string }>
}

export default async function BlogPostPage({ params }: BlogPostPageProps) {
  const { slug } = await params

  return (
    <ShaderBackground>
      <Header />
      <BlogPost slug={slug} />
    </ShaderBackground>
  )
}
