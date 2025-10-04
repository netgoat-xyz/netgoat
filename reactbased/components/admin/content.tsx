"use client";

import { OverviewContent } from "@/components/admin/content/overview";
import dynamic from "next/dynamic";
import { useState } from "react";

interface AdminContentProps {
  activeTab: string;
  activeSection: string;
  onTabChange: (tab: string) => void;
}

export function AdminContent({
  activeTab,
  activeSection,
  onTabChange,
}: AdminContentProps) {
  const [editSlug, setEditSlug] = useState<string | null>(null);

  const handleEdit = (slug: string) => {
    setEditSlug(slug);
    onTabChange("blog_edit");
  };

  if (activeTab === "overview") return <OverviewContent />;

  if (activeTab === "blog") {
    const BlogHome = dynamic(
      () => import("./content/blog").then((mod) => mod.default),
      { ssr: false },
    );
    return (
      <BlogHome
        onCreate={() => onTabChange("blog_create")}
        onEdit={handleEdit}
      />
    );
  }

  if (activeTab === "blog_create") {
    const BlogCreate = dynamic(
      () => import("./content/blog_create").then((mod) => mod.default),
      { ssr: false },
    );
    return <BlogCreate />;
  }

  if (activeTab === "blog_edit" && editSlug) {
    const BlogEdit = dynamic(
      () => import("./content/blog_edit").then((mod) => mod.default),
      { ssr: false },
    );
    return <BlogEdit slug={editSlug} />;
  }

  return (
    <div className="p-6 text-center py-12">
      <h2 className="text-xl font-semibold mb-2">
        {activeTab.charAt(0).toUpperCase() + activeTab.slice(1)} -{" "}
        {activeSection.charAt(0).toUpperCase() + activeSection.slice(1)}
      </h2>
      <p className="text-muted-foreground">
        Content for this section will be displayed here.
      </p>
    </div>
  );
}
