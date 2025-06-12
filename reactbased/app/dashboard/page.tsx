import { AppSidebar } from "@/components/home-sidebar"
import { ChartAreaInteractive } from "@/components/chart-area-interactive"
import { DataTable } from "@/components/domains-table"
import { SectionCards } from "@/components/section-cards"
import { SiteHeader } from "@/components/site-header"
import {
  SidebarInset,
  SidebarProvider,
} from "@/components/ui/sidebar"

import data from "./data.json"
const domainData: {
  group: string
  name: string
  status: "active" | "inactive" | "pending"
  lastSeen: string
}[] = [
  {
    group: "Cloudflare",
    name: "example.com",
    status: "active",
    lastSeen: "2025-06-11 10:42 AM",
  },
  {
    group: "User Created",
    name: "mycoolsite.dev",
    status: "inactive",
    lastSeen: "2025-06-10 02:19 PM",
  },
  {
    group: "Cloudflare",
    name: "nextgen.network",
    status: "pending",
    lastSeen: "2025-06-09 08:00 AM",
  },
  {
    group: "Local",
    name: "internal.service",
    status: "active",
    lastSeen: "2025-06-11 10:40 AM",
  },
  {
    group: "Cloudflare",
    name: "legacy.domain",
    status: "inactive",
    lastSeen: "2025-06-01 11:50 PM",
  },
]

export default function Page() {
  return (
    <SidebarProvider
      style={
        {
          "--sidebar-width": "calc(var(--spacing) * 72)",
          "--header-height": "calc(var(--spacing) * 12)",
        } as React.CSSProperties
      }
    >
      <AppSidebar variant="inset" />
      <SidebarInset>
        <SiteHeader />
        <div className="flex flex-1 flex-col">
          <div className="@container/main flex flex-1 flex-col gap-2">
            <div className="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
              <SectionCards />
              <div className="px-4 lg:px-6">
                              <DataTable data={domainData}/>

              </div>
            </div>
          </div>
        </div>
      </SidebarInset>
    </SidebarProvider>
  )
}
