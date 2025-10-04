"use client";

import { useState } from "react";
import { AdminHeader } from "@/components/admin/header";
import { AdminNav } from "@/components/admin/nav";
import { AdminSidebar } from "@/components/admin/sidebar";
import { AdminContent } from "@/components/admin/content";

export default function AdminPanel() {
  const [activeTab, setActiveTab] = useState("overview");
  const [activeSection, setActiveSection] = useState("general");

  return (
    <div className="min-h-screen bg-background">
      <AdminHeader />
      <AdminNav activeTab={activeTab} onTabChange={setActiveTab} />

      <div className="flex">
        {/* activeTab === "settings" && <AdminSidebar activeSection={activeSection} onSectionChange={setActiveSection} /> */}

        <main className="flex-1">
          <AdminContent
            activeTab={activeTab}
            onTabChange={setActiveTab}
            activeSection={activeSection}
          />
        </main>
      </div>
    </div>
  );
}
