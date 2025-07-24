"use client";

import { AppSidebar } from "@/components/domain-sidebar";
import { ChartAreaInteractive } from "@/components/chart-area-interactive";
import { DataTable } from "@/components/domains-table";
import { SectionCards } from "@/components/section-cards";
import SiteHeader from "@/components/site-header";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import { useEffect, useState } from "react";
import axios from "axios";

const cacheData = [/* your cacheData stays as-is */];

const isMobile = (ua: string) => /iPhone|iPad|iPod|Android|Mobile/i.test(ua);

export default function Page({ params }: { params: { domain: string; slug: string } }) {
  const [clients, setClients] = useState<any[]>([]);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const res = await axios.get(`${process.env.backendapi}/api/${params.slug}/analytics`, {
          headers: {
            Authorization: `Bearer ${localStorage.getItem("jwt")}`,
          },
        });

        const logs: { time: string; userAgent: string }[] = res.data;
        const dailyCounts: Record<string, { mobile: number; desktop: number }> = {};

        for (const log of logs) {
          const date = new Date(log.time).toISOString().slice(0, 10);
          if (!dailyCounts[date]) dailyCounts[date] = { mobile: 0, desktop: 0 };
          isMobile(log.userAgent) ? dailyCounts[date].mobile++ : dailyCounts[date].desktop++;
        }

        const finalData = Object.entries(dailyCounts).map(([date, counts]) => ({
          date,
          ...counts,
        }));

        setClients(finalData);
      } catch (err) {
        console.error("Failed to fetch analytics:", err);
      }
    };

    fetchData();
  }, [params.slug]);

  const cacheConfig = {
    cache: { label: "cache" },
    hit: { label: "Cache Hits", color: "var(--primary)" },
    miss: { label: "Cache Miss", color: "var(--primary)" },
  };

  const visitorConfig = {
    visitors: { label: "Visitors" },
    desktop: { label: "Desktop", color: "var(--primary)" },
    mobile: { label: "Mobile", color: "var(--primary)" },
  };

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
        <SiteHeader title={params.slug} />
        <div className="flex flex-1 flex-col">
          <div className="@container/main flex flex-1 flex-col gap-2">
            <div className="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
              <SectionCards />
              <div className="px-4 lg:px-6">
                <ChartAreaInteractive
                  title="Total visitors"
                  description="Total for the last 3 months"
                  cc={visitorConfig}
                  chartData={clients}
                  areaKeys={[
                    { key: "mobile", color: "var(--color-mobile)", gradient: "fillMobile" },
                    { key: "desktop", color: "var(--color-desktop)", gradient: "fillDesktop" },
                  ]}
                />
              </div>
              <div className="px-4 lg:px-6">
                <ChartAreaInteractive
                  title="Caching Total"
                  description="Cache data for the last 3 months"
                  cc={cacheConfig}
                  chartData={cacheData}
                  areaKeys={[
                    { key: "hit", color: "var(--color-hit)", gradient: "fillHit" },
                    { key: "miss", color: "var(--color-miss)", gradient: "fillMiss" },
                  ]}
                />
              </div>
            </div>
          </div>
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
