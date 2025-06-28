"use client";

import { AppSidebar } from "@/components/domain-sidebar";
import { ChartAreaInteractive } from "@/components/chart-area-interactive";
import { DataTable } from "@/components/dns-table";
import { SectionCards } from "@/components/section-cards";
import SiteHeader from "@/components/site-header";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import { CreateRecordSheet } from "@/components/DNS-Create-Record-Sheet";

export default function DNSPageContent({
  slug,
  data,
}: {
  slug: string;
  data: any[];
}) {
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
        <SiteHeader title={slug} />
        <div className="flex flex-1 flex-col">
          <div className="@container/main flex flex-1 flex-col gap-2">
            <div className="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
              <div className="px-4 lg:px-6">
                <div className="flex justify-between items-center">
                  <h1 className="font-inter font-bold text-white text-3xl">
                    DNS Records
                  </h1>
                  <CreateRecordSheet />
                </div>
                <DataTable data={data}></DataTable>
              </div>
            </div>
          </div>
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
