"use client"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Search } from "lucide-react"
import { cn } from "@/lib/utils"

interface AdminSidebarProps {
  activeSection: string
  onSectionChange: (section: string) => void
}

const sidebarSections = [
  { id: "general", label: "General" },
  { id: "build-deployment", label: "Build and Deployment" },
  { id: "domains", label: "Domains" },
  { id: "environments", label: "Environments" },
  { id: "environment-variables", label: "Environment Variables" },
  { id: "git", label: "Git" },
  { id: "integrations", label: "Integrations" },
  { id: "deployment-protection", label: "Deployment Protection" },
  { id: "functions", label: "Functions" },
  { id: "data-cache", label: "Data Cache" },
  { id: "cron-jobs", label: "Cron Jobs" },
  { id: "microfrontends", label: "Microfrontends" },
  { id: "project-members", label: "Project Members" },
  { id: "webhooks", label: "Webhooks" },
  { id: "log-drains", label: "Log Drains" },
  { id: "security", label: "Security" },
  { id: "secure-compute", label: "Secure Compute" },
  { id: "advanced", label: "Advanced" },
]

export function AdminSidebar({ activeSection, onSectionChange }: AdminSidebarProps) {
  return (
    <aside className="w-64 border-r border-border bg-card">
      <div className="p-4">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input placeholder="Search..." className="pl-9 bg-background" />
        </div>
      </div>

      <ScrollArea className="h-[calc(100vh-8rem)]">
        <div className="p-2">
          {sidebarSections.map((section) => (
            <Button
              key={section.id}
              variant="ghost"
              className={cn(
                "w-full justify-start text-sm font-normal mb-1",
                activeSection === section.id && "bg-accent text-accent-foreground",
              )}
              onClick={() => onSectionChange(section.id)}
            >
              {section.label}
            </Button>
          ))}
        </div>
      </ScrollArea>
    </aside>
  )
}
