import { AppSidebar } from "@/components/domain-sidebar";
import { ChartAreaInteractive } from "@/components/chart-area-interactive";
import { DataTable } from "@/components/domains-table";
import { SectionCards } from "@/components/section-cards";
import SiteHeader from "@/components/site-header";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";

const clients = [
  { date: "2024-04-01", desktop: 222, mobile: 150 },
  { date: "2024-04-02", desktop: 97, mobile: 180 },
  { date: "2024-04-03", desktop: 167, mobile: 120 },
  { date: "2024-04-04", desktop: 242, mobile: 260 },
  { date: "2024-04-05", desktop: 373, mobile: 290 },
  { date: "2024-04-06", desktop: 301, mobile: 340 },
  { date: "2024-04-07", desktop: 245, mobile: 180 },
  { date: "2024-04-08", desktop: 409, mobile: 320 },
  { date: "2024-04-09", desktop: 59, mobile: 110 },
  { date: "2024-04-10", desktop: 261, mobile: 190 },
  { date: "2024-04-11", desktop: 327, mobile: 350 },
  { date: "2024-04-12", desktop: 292, mobile: 210 },
  { date: "2024-04-13", desktop: 342, mobile: 380 },
  { date: "2024-04-14", desktop: 137, mobile: 220 },
  { date: "2024-04-15", desktop: 120, mobile: 170 },
  { date: "2024-04-16", desktop: 138, mobile: 190 },
  { date: "2024-04-17", desktop: 446, mobile: 360 },
  { date: "2024-04-18", desktop: 364, mobile: 410 },
  { date: "2024-04-19", desktop: 243, mobile: 180 },
  { date: "2024-04-20", desktop: 89, mobile: 150 },
  { date: "2024-04-21", desktop: 137, mobile: 200 },
  { date: "2024-04-22", desktop: 224, mobile: 170 },
  { date: "2024-04-23", desktop: 138, mobile: 230 },
  { date: "2024-04-24", desktop: 387, mobile: 290 },
  { date: "2024-04-25", desktop: 215, mobile: 250 },
  { date: "2024-04-26", desktop: 75, mobile: 130 },
  { date: "2024-04-27", desktop: 383, mobile: 420 },
  { date: "2024-04-28", desktop: 122, mobile: 180 },
  { date: "2024-04-29", desktop: 315, mobile: 240 },
  { date: "2024-04-30", desktop: 454, mobile: 380 },
  { date: "2024-05-01", desktop: 165, mobile: 220 },
  { date: "2024-05-02", desktop: 293, mobile: 310 },
  { date: "2024-05-03", desktop: 247, mobile: 190 },
  { date: "2024-05-04", desktop: 385, mobile: 420 },
  { date: "2024-05-05", desktop: 481, mobile: 390 },
  { date: "2024-05-06", desktop: 498, mobile: 520 },
  { date: "2024-05-07", desktop: 388, mobile: 300 },
  { date: "2024-05-08", desktop: 149, mobile: 210 },
  { date: "2024-05-09", desktop: 227, mobile: 180 },
  { date: "2024-05-10", desktop: 293, mobile: 330 },
  { date: "2024-05-11", desktop: 335, mobile: 270 },
  { date: "2024-05-12", desktop: 197, mobile: 240 },
  { date: "2024-05-13", desktop: 197, mobile: 160 },
  { date: "2024-05-14", desktop: 448, mobile: 490 },
  { date: "2024-05-15", desktop: 473, mobile: 380 },
  { date: "2024-05-16", desktop: 338, mobile: 400 },
  { date: "2024-05-17", desktop: 499, mobile: 420 },
  { date: "2024-05-18", desktop: 315, mobile: 350 },
  { date: "2024-05-19", desktop: 235, mobile: 180 },
  { date: "2024-05-20", desktop: 177, mobile: 230 },
  { date: "2024-05-21", desktop: 82, mobile: 140 },
  { date: "2024-05-22", desktop: 81, mobile: 120 },
  { date: "2024-05-23", desktop: 252, mobile: 290 },
  { date: "2024-05-24", desktop: 294, mobile: 220 },
  { date: "2024-05-25", desktop: 201, mobile: 250 },
  { date: "2024-05-26", desktop: 213, mobile: 170 },
  { date: "2024-05-27", desktop: 420, mobile: 460 },
  { date: "2024-05-28", desktop: 233, mobile: 190 },
  { date: "2024-05-29", desktop: 78, mobile: 130 },
  { date: "2024-05-30", desktop: 340, mobile: 280 },
  { date: "2024-05-31", desktop: 178, mobile: 230 },
  { date: "2024-06-01", desktop: 178, mobile: 200 },
  { date: "2024-06-02", desktop: 470, mobile: 410 },
  { date: "2024-06-03", desktop: 103, mobile: 160 },
  { date: "2024-06-04", desktop: 439, mobile: 380 },
  { date: "2024-06-05", desktop: 88, mobile: 140 },
  { date: "2024-06-06", desktop: 294, mobile: 250 },
  { date: "2024-06-07", desktop: 323, mobile: 370 },
  { date: "2024-06-08", desktop: 385, mobile: 320 },
  { date: "2024-06-09", desktop: 438, mobile: 480 },
  { date: "2024-06-10", desktop: 155, mobile: 200 },
  { date: "2024-06-11", desktop: 92, mobile: 150 },
  { date: "2024-06-12", desktop: 492, mobile: 420 },
  { date: "2024-06-13", desktop: 81, mobile: 130 },
  { date: "2024-06-14", desktop: 426, mobile: 380 },
  { date: "2024-06-15", desktop: 307, mobile: 350 },
  { date: "2024-06-16", desktop: 371, mobile: 310 },
  { date: "2024-06-17", desktop: 475, mobile: 520 },
  { date: "2024-06-18", desktop: 107, mobile: 170 },
  { date: "2024-06-19", desktop: 341, mobile: 290 },
  { date: "2024-06-20", desktop: 408, mobile: 450 },
  { date: "2024-06-21", desktop: 169, mobile: 210 },
  { date: "2024-06-22", desktop: 317, mobile: 270 },
  { date: "2024-06-23", desktop: 480, mobile: 530 },
  { date: "2024-06-24", desktop: 132, mobile: 180 },
  { date: "2024-06-25", desktop: 141, mobile: 190 },
  { date: "2024-06-26", desktop: 434, mobile: 380 },
  { date: "2024-06-27", desktop: 448, mobile: 490 },
  { date: "2024-06-28", desktop: 149, mobile: 200 },
  { date: "2024-06-29", desktop: 103, mobile: 160 },
  { date: "2024-06-30", desktop: 446, mobile: 400 },
]


const cacheData = [
  { date: "2024-04-01", miss: 222, hit: 150 },
  { date: "2024-04-02", miss: 97, hit: 180 },
  { date: "2024-04-03", miss: 167, hit: 120 },
  { date: "2024-04-04", miss: 242, hit: 260 },
  { date: "2024-04-05", miss: 373, hit: 290 },
  { date: "2024-04-06", miss: 301, hit: 340 },
  { date: "2024-04-07", miss: 245, hit: 180 },
  { date: "2024-04-08", miss: 409, hit: 320 },
  { date: "2024-04-09", miss: 59, hit: 110 },
  { date: "2024-04-10", miss: 261, hit: 190 },
  { date: "2024-04-11", miss: 327, hit: 350 },
  { date: "2024-04-12", miss: 292, hit: 210 },
  { date: "2024-04-13", miss: 342, hit: 380 },
  { date: "2024-04-14", miss: 137, hit: 220 },
  { date: "2024-04-15", miss: 120, hit: 170 },
  { date: "2024-04-16", miss: 138, hit: 190 },
  { date: "2024-04-17", miss: 446, hit: 360 },
  { date: "2024-04-18", miss: 364, hit: 410 },
  { date: "2024-04-19", miss: 243, hit: 180 },
  { date: "2024-04-20", miss: 89, hit: 150 },
  { date: "2024-04-21", miss: 137, hit: 200 },
  { date: "2024-04-22", miss: 224, hit: 170 },
  { date: "2024-04-23", miss: 138, hit: 230 },
  { date: "2024-04-24", miss: 387, hit: 290 },
  { date: "2024-04-25", miss: 215, hit: 250 },
  { date: "2024-04-26", miss: 75, hit: 130 },
  { date: "2024-04-27", miss: 383, hit: 420 },
  { date: "2024-04-28", miss: 122, hit: 180 },
  { date: "2024-04-29", miss: 315, hit: 240 },
  { date: "2024-04-30", miss: 454, hit: 380 },
  { date: "2024-05-01", miss: 165, hit: 220 },
  { date: "2024-05-02", miss: 293, hit: 310 },
  { date: "2024-05-03", miss: 247, hit: 190 },
  { date: "2024-05-04", miss: 385, hit: 420 },
  { date: "2024-05-05", miss: 481, hit: 390 },
  { date: "2024-05-06", miss: 498, hit: 520 },
  { date: "2024-05-07", miss: 388, hit: 300 },
  { date: "2024-05-08", miss: 149, hit: 210 },
  { date: "2024-05-09", miss: 227, hit: 180 },
  { date: "2024-05-10", miss: 293, hit: 330 },
  { date: "2024-05-11", miss: 335, hit: 270 },
  { date: "2024-05-12", miss: 197, hit: 240 },
  { date: "2024-05-13", miss: 197, hit: 160 },
  { date: "2024-05-14", miss: 448, hit: 490 },
  { date: "2024-05-15", miss: 473, hit: 380 },
  { date: "2024-05-16", miss: 338, hit: 400 },
  { date: "2024-05-17", miss: 499, hit: 420 },
  { date: "2024-05-18", miss: 315, hit: 350 },
  { date: "2024-05-19", miss: 235, hit: 180 },
  { date: "2024-05-20", miss: 177, hit: 230 },
  { date: "2024-05-21", miss: 82, hit: 140 },
  { date: "2024-05-22", miss: 81, hit: 120 },
  { date: "2024-05-23", miss: 252, hit: 290 },
  { date: "2024-05-24", miss: 294, hit: 220 },
  { date: "2024-05-25", miss: 201, hit: 250 },
  { date: "2024-05-26", miss: 213, hit: 170 },
  { date: "2024-05-27", miss: 420, hit: 460 },
  { date: "2024-05-28", miss: 233, hit: 190 },
  { date: "2024-05-29", miss: 78, hit: 130 },
  { date: "2024-05-30", miss: 340, hit: 280 },
  { date: "2024-05-31", miss: 178, hit: 230 },
  { date: "2024-06-01", miss: 178, hit: 200 },
  { date: "2024-06-02", miss: 470, hit: 410 },
  { date: "2024-06-03", miss: 103, hit: 160 },
  { date: "2024-06-04", miss: 439, hit: 380 },
  { date: "2024-06-05", miss: 88, hit: 140 },
  { date: "2024-06-06", miss: 294, hit: 250 },
  { date: "2024-06-07", miss: 323, hit: 370 },
  { date: "2024-06-08", miss: 385, hit: 320 },
  { date: "2024-06-09", miss: 438, hit: 480 },
  { date: "2024-06-10", miss: 155, hit: 200 },
  { date: "2024-06-11", miss: 92, hit: 150 },
  { date: "2024-06-12", miss: 492, hit: 420 },
  { date: "2024-06-13", miss: 81, hit: 130 },
  { date: "2024-06-14", miss: 426, hit: 380 },
  { date: "2024-06-15", miss: 307, hit: 350 },
  { date: "2024-06-16", miss: 371, hit: 310 },
  { date: "2024-06-17", miss: 475, hit: 520 },
  { date: "2024-06-18", miss: 107, hit: 170 },
  { date: "2024-06-19", miss: 341, hit: 290 },
  { date: "2024-06-20", miss: 408, hit: 450 },
  { date: "2024-06-21", miss: 169, hit: 210 },
  { date: "2024-06-22", miss: 317, hit: 270 },
  { date: "2024-06-23", miss: 480, hit: 530 },
  { date: "2024-06-24", miss: 132, hit: 180 },
  { date: "2024-06-25", miss: 141, hit: 190 },
  { date: "2024-06-26", miss: 434, hit: 380 },
  { date: "2024-06-27", miss: 448, hit: 490 },
  { date: "2024-06-28", miss: 149, hit: 200 },
  { date: "2024-06-29", miss: 103, hit: 160 },
  { date: "2024-06-30", miss: 446, hit: 400 },
]

export default async function Page({
  params,
}: {
  params: Promise<{
    domain: String;
    slug: string;
  }>;
}) {

const cacheConfig = {
  cache: {
    label: "cache",
  },
  hit: {
    label: "Cache Hits",
    color: "var(--primary)",
  },
  miss: {
    label: "Cache Miss",
    color: "var(--primary)",
  },
}

const visitorConfig = {
  visitors: {
    label: "Visitors",
  },
  desktop: {
    label: "Desktop",
    color: "var(--primary)",
  },
  mobile: {
    label: "Mobile",
    color: "var(--primary)",
  },
}

  const slug = await params;
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
        <SiteHeader title={slug.slug} />
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
