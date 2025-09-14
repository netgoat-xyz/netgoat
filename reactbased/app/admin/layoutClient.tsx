"use client"

import { useEffect, useState } from "react"
import { usePathname, useRouter } from "next/navigation"
import { AnimatePresence, motion } from "framer-motion"
import Link from "next/link"

export default function AdminLayout({ children }: { children: React.ReactNode }) {
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
        Loading admin...
      </div>
    )
  }

  return (
    <div className="flex flex-col h-screen">
      {/* topbar */}
      <header className="flex h-14 items-center justify-between border-b px-6">
        <div className="flex items-center gap-6">
          <Link href="/admin" className="font-semibold">
            Admin
          </Link>
          <nav className="flex gap-4 text-sm text-muted-foreground">
            <Link
              href="/admin/users"
              className={pathname.startsWith("/admin/users") ? "text-foreground font-medium" : ""}
            >
              Users
            </Link>
            <Link
              href="/admin/logs"
              className={pathname.startsWith("/admin/logs") ? "text-foreground font-medium" : ""}
            >
              Logs
            </Link>
            <Link
              href="/admin/settings"
              className={pathname.startsWith("/admin/settings") ? "text-foreground font-medium" : ""}
            >
              Settings
            </Link>
          </nav>
        </div>
        <div className="flex items-center gap-3">
          <button className="text-sm text-muted-foreground hover:text-foreground">Search</button>
          <button className="text-sm text-muted-foreground hover:text-foreground">Profile</button>
        </div>
      </header>

      {/* animated content */}
      <main className="flex-1 overflow-y-auto p-6">
        <AnimatePresence mode="wait">
          <motion.div
            key={pathname}
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -16 }}
            transition={{ duration: 0.2, ease: "easeInOut" }}
            className="h-full w-full"
          >
            {children}
          </motion.div>
        </AnimatePresence>
      </main>
    </div>
  )
}
