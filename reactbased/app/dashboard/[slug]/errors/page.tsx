import { SectionCards } from "@/components/errors-cards";
import ErrorsTable from "@/components/errors-table";

const data: {
  name: string;
  site: string;
  users: number;
  id: string
  lastSeen: string;
  status: "patching" | "fixed" | "on-going";
}[] = [
  {
    name: "Undefined Error",
    id: "3",
    site: "example.com",
    users: 212,
    lastSeen: "2023-10-01T12:00:00Z",
    status: "patching",
  },
  {
    name: "Undefined Error",
    id: "2",
    site: "example.com",
    users: 212,
    lastSeen: "2023-10-01T12:00:00Z",
    status: "fixed",
  },
  {
    name: "Undefined Error",
    id: "1",
    site: "example.com",
    users: 212,
    lastSeen: "2023-10-01T12:00:00Z",
    status: "on-going",
  },
];

export default async function Page({
  params,
}: {
  params: Promise<{
    domain: string;
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
  };

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
  };

  const slug = await params;
  return (
    <div className="flex flex-1 flex-col">
      <div className="@container/main flex flex-1 flex-col gap-2">
        <div className="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
          <SectionCards />
          <div className="px-4 lg:px-6">
            <ErrorsTable data={data} />
          </div>
        </div>
      </div>
    </div>
  );
}
