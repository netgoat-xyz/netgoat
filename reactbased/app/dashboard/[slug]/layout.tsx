// app/dashboard/[slug]/layout.tsx
import { Metadata } from "next"
import { ReactNode } from "react"
import DashboardClientWrapper from "../layoutClient"
import { Toaster } from "@/components/ui/sonner"

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const formattedSlug = (await params).slug.charAt(0).toUpperCase() + (await params).slug.slice(1)

  return {
    title: `${formattedSlug} | Dashboard`,
    description: `Dashboard section: ${formattedSlug}`,
  }
}

export default function SlugLayout({
  children,
  params,
}: {
  children: ReactNode
  params: Promise<{ slug: string }> 
}) {
  return (
    <DashboardClientWrapper params={params}>
      {children}
      <Toaster />
    </DashboardClientWrapper>
  )
}
