"use client"

import { useEffect, useState } from "react"
import { usePathname, useRouter } from "next/navigation"
import { AnimatePresence, motion } from "framer-motion"
import { AppSidebar } from "@/components/domain-sidebar";
import SiteHeader from "@/components/site-header";
import { SidebarProvider, SidebarInset } from "@/components/ui/sidebar";

export default function DashboardClientWrapper({ children, params }: { children: React.ReactNode, params?: any }) {
  const router = useRouter()
  const pathname = usePathname()
  const [ready, setReady] = useState(false)

  useEffect(() => {
    const jwt = localStorage.getItem("jwt")
    if (!jwt) {
      router.replace("/auth")
      return
    }
    fetch("/api/session?session=" + jwt)
      .then((res) => res.json())
      .then((data) => {
        localStorage.setItem("session", JSON.stringify(data))
        setReady(true)
      })
      .catch(() => router.replace("/auth"))
  }, [router])

  if (!ready) {
    return (
      <div className="flex h-screen items-center justify-center text-muted-foreground">
        Loading dashboard...
      </div>
    )
  }

  return (
    <SidebarProvider
      style={{
        "--sidebar-width": "calc(var(--spacing) * 72)",
        "--header-height": "calc(var(--spacing) * 12)",
      } as React.CSSProperties}
      id="DashboardSidebarProvider"
    >
      <AppSidebar variant="inset" id="AppSidebar" />
      <SidebarInset id="SidebarInset">
        <SiteHeader title={params?.slug ?? "Dashboard"} id="SiteHeader" />

        {/* animated page slot */}
        <AnimatePresence mode="wait">
          <motion.div
            key={pathname}
            initial={{ opacity: 0, y: 24 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -24 }}
            transition={{ duration: 0.25, ease: "easeInOut" }}
            className="h-full w-full"
          >
            {children}
          </motion.div>
        </AnimatePresence>
      </SidebarInset>
    </SidebarProvider>
  )
}
