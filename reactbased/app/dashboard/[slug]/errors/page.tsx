import { AppSidebar } from "@/components/domain-sidebar";
import { ChartAreaInteractive } from "@/components/chart-area-interactive";
import { SectionCards } from "@/components/errors-cards";
import ErrorsTable from "@/components/errors-table";
import SiteHeader from "@/components/site-header";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";

const data = [
  {
    name: "Undefined Error",
    site: "example.com",
    users: 212,
    lastSeen: "2023-10-01T12:00:00Z",
    status: "patching" as "patching",
  },
    {
    name: "Undefined Error",
    site: "example.com",
    users: 212,
    lastSeen: "2023-10-01T12:00:00Z",
    status: "fixed" as "fixed",
  },
    {
    name: "Undefined Error",
    site: "example.com",
    users: 212,
    lastSeen: "2023-10-01T12:00:00Z",
    status: "on-going" as "on-going",
  },
];

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
        <div className="flex flex-1 flex-col">
          <div className="@container/main flex flex-1 flex-col gap-2">
            <div className="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
              <SectionCards />
              <div className="px-4 lg:px-6">
                <ErrorsTable data={data}/>
              </div>
            </div>
          </div>
        </div>
  );
}
