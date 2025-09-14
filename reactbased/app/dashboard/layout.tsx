// app/dashboard/layout.tsx
import { Metadata } from "next";
import { ReactNode } from "react";
import DashboardClientWrapper from "./layoutClient"; // your "use client" wrapper
<<<<<<< HEAD
import { Toaster } from "@/components/ui/sonner";
=======
>>>>>>> 1e26c937094b9bb52e60e9b85f0514df46ed7c2d

interface LayoutProps {
  children: ReactNode;
  params?: { slug?: string; section?: string };
}

// Server Component
<<<<<<< HEAD
export async function generateMetadata({
  params,
}: LayoutProps): Promise<Metadata> {
=======
export async function generateMetadata({ params }: LayoutProps): Promise<Metadata> {
>>>>>>> 1e26c937094b9bb52e60e9b85f0514df46ed7c2d
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

<<<<<<< HEAD
export default function DashboardLayout({ children, params }: LayoutProps) {
  return (
    <DashboardClientWrapper params={params}>
      {children} <Toaster />
    </DashboardClientWrapper>
=======

export default function DashboardLayout({ children, params }: LayoutProps) {
  return (
    <DashboardClientWrapper params={params}>{children}</DashboardClientWrapper>
>>>>>>> 1e26c937094b9bb52e60e9b85f0514df46ed7c2d
  );
}
