"use client"

import { useState, useEffect } from "react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu"
import { Bell, HelpCircle, FileText, MessageSquare, ChevronDown } from "lucide-react"

export function AdminHeader() {
  const [notifications, setNotifications] = useState<string[]>([])

  // Demo: simulate fetching notifications
  useEffect(() => {
    const timer = setInterval(() => {
      setNotifications((prev) => [...prev, `Notification ${prev.length + 1}`])
    }, 10000) // every 10s add a notification
    return () => clearInterval(timer)
  }, [])

  const username = JSON.parse(localStorage.getItem("session") ?? "{}").username ?? "JD"

  return (
    <header className="border-b border-border bg-background">
      <div className="flex h-14 items-center justify-between px-6">
        {/* Left side - Logo */}
        <h1 className="text-2xl font-calsans text-foreground">Netgoat</h1>

        {/* Right side - Actions */}
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="sm">
            <FileText className="h-4 w-4" />
            <span className="text-sm">Docs</span>
          </Button>

          {/* Notifications */}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="relative">
                <Bell className="h-4 w-4" />
                {notifications.length > 0 && (
                  <span className="absolute -top-1 -right-1 h-2 w-2 bg-primary rounded-full animate-pulse"></span>
                )}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-64">
              {notifications.length === 0 && (
                <DropdownMenuItem className="text-muted-foreground">
                  No new notifications
                </DropdownMenuItem>
              )}
              {notifications.map((n, i) => (
                <DropdownMenuItem key={i}>{n}</DropdownMenuItem>
              ))}
              {notifications.length > 0 && <DropdownMenuSeparator />}
              {notifications.length > 0 && (
                <DropdownMenuItem
                  onClick={() => setNotifications([])}
                  className="text-red-500"
                >
                  Clear all
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" className="rounded-full gap-2 p-0">
                <Avatar className="h-8 w-8">
                  <AvatarImage
                    src={`https://www.tapback.co/api/avatar/${username}`}
                    alt={username}
                  />
                  <AvatarFallback>{username[0]}</AvatarFallback>
                </Avatar>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem>Profile</DropdownMenuItem>
              <DropdownMenuItem>Settings</DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem>Sign out</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </header>
  )
}
