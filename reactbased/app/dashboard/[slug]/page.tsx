"use client";

import { ChartAreaInteractive } from "@/components/chart-area-interactive";
import { useEffect, useState } from "react";
import axios from "axios";
import { SectionCards } from "@/components/section-cards";

interface DashboardPageProps {
  params: Promise<{ domain: string; slug: string }>;
}

interface ClientData {
  date: string;
  mobile: number;
  desktop: number;
}

interface CacheData {
  date: string;
  hit: number;
  miss: number;
}

const cacheData: CacheData[] = [
  { date: "2025-06-01", hit: 100, miss: 20 },
  { date: "2025-06-02", hit: 120, miss: 25 },
  { date: "2025-06-03", hit: 150, miss: 30 },
];

const isMobile = (ua: string) => /iPhone|iPad|iPod|Android|Mobile/i.test(ua);

export default function DashboardPage({ params }: DashboardPageProps) {
  const [slug, setSlug] = useState<string>("");
  const [clients, setClients] = useState<ClientData[]>([]);
  const [timeRange, setTimeRange] = useState<string>("3mo");
  const [isLoading, setIsLoading] = useState(true);
  const [isClient, setIsClient] = useState(false);

  // Set client-side flag - this runs only on client
  useEffect(() => {
    setIsClient(true);
  }, []);

  // Handle async params
  useEffect(() => {
    const resolveParams = async () => {
      const resolvedParams = await params;
      setSlug(resolvedParams.slug);
    };
    resolveParams();
  }, [params]);

  useEffect(() => {
    // Don't run on server side
    if (!isClient || !slug) return;

    const fetchData = async () => {
      try {
        setIsLoading(true);
        
        // Now we're sure we're on client side
        const jwt = localStorage.getItem("jwt");

        if (!jwt) {
          console.error("No JWT token found");
          setIsLoading(false);
          return;
        }

        // Use environment variable with fallback
        const logdbUrl = process.env.NEXT_PUBLIC_LOGDB || "http://localhost:3001";

        const res = await axios.get<{ time: string; userAgent: string }[]>(
          `${logdbUrl}/api/${slug}/analytics?timeframe=${timeRange}`,
          {
            headers: {
              Authorization: `Bearer ${jwt}`,
            },
          },
        );

        const logs = res.data;
        const dailyCounts: Record<string, { mobile: number; desktop: number }> = {};

        for (const log of logs) {
          const date = new Date(log.time).toISOString().slice(0, 10);
          if (!dailyCounts[date]) dailyCounts[date] = { mobile: 0, desktop: 0 };
          isMobile(log.userAgent)
            ? dailyCounts[date].mobile++
            : dailyCounts[date].desktop++;
        }

        const finalData = Object.entries(dailyCounts).map(([date, counts]) => ({
          date,
          ...counts,
        }));

        setClients(finalData);
      } catch (err) {
        console.error("Failed to fetch analytics:", err);
      } finally {
        setIsLoading(false);
      }
    };

    fetchData();
  }, [slug, timeRange, isClient]);

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

  // Show loading state while slug is being resolved or on server
  if (!isClient || !slug || isLoading) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center">
        <div className="text-muted-foreground">Loading analytics...</div>
      </div>
    );
  }

  return (
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
                {
                  key: "mobile",
                  color: "var(--color-mobile)",
                  gradient: "fillMobile",
                },
                {
                  key: "desktop",
                  color: "var(--color-desktop)",
                  gradient: "fillDesktop",
                },
              ]}
              timeRange={timeRange}
              setTimeRange={setTimeRange}
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
                {
                  key: "miss",
                  color: "var(--color-miss)",
                  gradient: "fillMiss",
                },
              ]}
              timeRange="3mo"
              setTimeRange={() => {}}
            />
          </div>
        </div>
      </div>
    </div>
  );
}