// app/dashboard/layout.tsx
import { Metadata } from "next";
import { ReactNode } from "react";
import DashboardClientWrapper from "./layoutClient"; // your "use client" wrapper

interface LayoutProps {
  children: ReactNode;
  params?: { slug?: string; section?: string };
}

// Server Component
export async function generateMetadata({ params }: LayoutProps): Promise<Metadata> {
  const slug = params?.slug ?? "Dashboard";

  // extract the last segment of the path for "section"
  const sectionSegment = params?.section ?? ""; // might be undefined
  const section = sectionSegment || "Overview";

  const formattedSection = section.charAt(0).toUpperCase() + section.slice(1);

  return {
    title: `${slug} | ${formattedSection}`,
    description: `Dashboard for ${slug} - ${formattedSection}`,
  };
}


export default function DashboardLayout({ children, params }: LayoutProps) {
  return (
    <DashboardClientWrapper>{children}</DashboardClientWrapper>
  );
}
