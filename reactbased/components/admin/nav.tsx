"use client"

import { useRef, useState, useLayoutEffect } from "react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { motion } from "framer-motion"

interface AdminNavProps {
  activeTab: string
  onTabChange: (tab: string) => void
}

const navItems = [
  { id: "overview", label: "Overview" },
  { id: "users", label: "Users" },
  { id: "domains-hosted", label: "Domains Hosted" },
  { id: "analytics", label: "Analytics" },
  { id: "speed-insights", label: "Speed Insights" },
  { id: "logs", label: "Global Logs" },
  { id: "nodes", label: "Node Stats" },
  { id: "database-viewer", label: "DB Viewer" },
  { id: "support", label: "Support Tickets" },
  { id: "AI", label: "AI" },
  { id: "blog", label: "Blog" },
  { id: "settings", label: "Settings" },
]

export function AdminNav({ activeTab, onTabChange }: AdminNavProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [underlineStyle, setUnderlineStyle] = useState({ left: 0, width: 0 })

  // normalize blog-related tabs
  const normalizedTab = activeTab.startsWith("blog") ? "blog" : activeTab

  useLayoutEffect(() => {
    if (!containerRef.current) return
    const buttons = Array.from(containerRef.current.querySelectorAll("button"))
    const activeButton = buttons.find(
      (btn) => btn.getAttribute("data-id") === normalizedTab
    )
    if (activeButton) {
      const rect = activeButton.getBoundingClientRect()
      const containerRect = containerRef.current.getBoundingClientRect()
      setUnderlineStyle({
        left: rect.left - containerRect.left,
        width: rect.width,
      })
    }
  }, [normalizedTab])

  return (
    <nav className="border-b border-border bg-background">
      <div ref={containerRef} className="relative flex items-center overflow-x-auto px-4">
        {navItems.map((item) => {
          const isActive = normalizedTab === item.id
          return (
            <Button
              key={item.id}
              data-id={item.id}
              variant="ghost"
              className={cn(
                "relative z-0 h-12 px-4 py-2 text-sm font-medium",
                "hover:text-primary hover:bg-background",
                isActive ? "text-primary" : "text-muted-foreground"
              )}
              onClick={() => onTabChange(item.id)}
            >
              {item.label}
            </Button>
          )
        })}

        <motion.div
          layout
          initial={false}
          animate={{ left: underlineStyle.left, width: underlineStyle.width }}
          transition={{ type: "spring", stiffness: 250, damping: 30 }}
          className="absolute bottom-0 h-[2px] bg-primary z-10 rounded-full"
        />
      </div>
    </nav>
  )
}
