"use client";

import React, { useState } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { AppSidebar } from "@/components/domain-sidebar";
import SiteHeader from "@/components/site-header";
import { SidebarProvider, SidebarInset } from "@/components/ui/sidebar";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/components/ui/table";
import {
  ChartContainer,
} from "@/components/ui/chart";
import {
  BarChart as ReBarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip as ReTooltip,
  ResponsiveContainer,
  LineChart,
  Line,
  CartesianGrid,
} from "recharts";
import { TooltipProps } from "recharts";

function DarkTooltip({ active, payload, label }: TooltipProps<any, any>) {
  if (!active || !payload || !payload.length) return null;
  return (
    <div style={{
      background: "#18181b",
      color: "#fff",
      borderRadius: 8,
      padding: "8px 12px",
      boxShadow: "0 2px 8px rgba(0,0,0,0.25)",
      border: "1px solid #27272a",
      fontSize: 14,
      minWidth: 120,
    }}>
      <div style={{ fontWeight: 600, marginBottom: 4 }}>{label}</div>
      {payload.map((entry: any, i: number) => (
        <div key={i} style={{ display: "flex", justifyContent: "space-between" }}>
          <span>{entry.name || entry.dataKey}</span>
          <span style={{ color: entry.color }}>{entry.value}</span>
        </div>
      ))}
    </div>
  );
}

function RequestBarChart({ data }: { data: { name: string; value: number }[] }) {
  return (
    <ChartContainer config={{ requests: { color: '#6366f1', label: 'Requests' } }}>
      <ResponsiveContainer width="100%" height={300}>
        <ReBarChart data={data} margin={{ top: 16, right: 16, left: 0, bottom: 0 }}>
          <XAxis dataKey="name" stroke="#fff" />
          <YAxis stroke="#fff" />
          <CartesianGrid strokeDasharray="3 3" stroke="#444" />
          <ReTooltip content={<DarkTooltip />} />
          <Bar dataKey="value" fill="#6366f1" />
        </ReBarChart>
      </ResponsiveContainer>
    </ChartContainer>
  );
}

function ErrorLineChart({ data }: { data: { name: string; value: number }[] }) {
  return (
    <ChartContainer config={{ errors: { color: '#ef4444', label: 'Errors' } }}>
      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={data} margin={{ top: 16, right: 16, left: 0, bottom: 0 }}>
          <XAxis dataKey="name" stroke="#fff" />
          <YAxis stroke="#fff" />
          <CartesianGrid strokeDasharray="3 3" stroke="#444" />
          <ReTooltip content={<DarkTooltip />} />
          <Line type="monotone" dataKey="value" stroke="#ef4444" strokeWidth={2} />
        </LineChart>
      </ResponsiveContainer>
    </ChartContainer>
  );
}

function DataTable() {
  const data = [
    { subdomain: "api.example.com", count: 12000 },
    { subdomain: "cdn.example.com", count: 9500 },
    { subdomain: "auth.example.com", count: 8800 },
  ];
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Subdomain</TableHead>
          <TableHead>Requests</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {data.map((row, i) => (
          <TableRow key={i}>
            <TableCell>{row.subdomain}</TableCell>
            <TableCell>{row.count.toLocaleString()}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

export default function AnalyticsPage({
  params,
}: {
  params: Promise<{ domain: string; slug: string }>;
}) {
  const [timeframe, setTimeframe] = useState("24h");
  const { slug } = React.use(params);

  // Example data for charts
  const barData = Array.from({ length: 10 }, (_, i) => ({ name: `Day ${i + 1}`, value: Math.floor(Math.random() * 1000) }));
  const lineData = Array.from({ length: 10 }, (_, i) => ({ name: `Day ${i + 1}`, value: Math.floor(Math.random() * 100) }));

  return (
    <SidebarProvider
      style={{
        "--sidebar-width": "calc(var(--spacing) * 72)",
        "--header-height": "calc(var(--spacing) * 12)",
      } as React.CSSProperties}
    >
      <AppSidebar variant="inset" />
      <SidebarInset>
        <SiteHeader title={slug} />
        <div className="flex flex-1 flex-col">
          <div className="@container/main flex flex-1 flex-col gap-2">
            <div className="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
              <div className="p-6 space-y-6">
                <div className="flex items-center justify-between flex-wrap gap-2">
                  <h1 className="text-3xl font-bold text-white">Analytics</h1>
                  <Tabs defaultValue={timeframe} onValueChange={setTimeframe}>
                    <TabsList>
                      <TabsTrigger value="24h">24H</TabsTrigger>
                      <TabsTrigger value="7d">7D</TabsTrigger>
                      <TabsTrigger value="30d">30D</TabsTrigger>
                    </TabsList>
                  </Tabs>
                </div>
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                  <Card>
                    <CardContent>
                      <h2 className="text-xl font-semibold text-white mb-2">Requests Over Time</h2>
                      <RequestBarChart data={barData} />
                    </CardContent>
                  </Card>
                  <Card>
                    <CardContent>
                      <h2 className="text-xl font-semibold text-white mb-2">Error Rates</h2>
                      <ErrorLineChart data={lineData} />
                    </CardContent>
                  </Card>
                </div>
                <Card>
                  <CardContent>
                    <h2 className="text-xl font-semibold text-white mb-2">Top Subdomains by Request</h2>
                    <DataTable />
                  </CardContent>
                </Card>
              </div>
            </div>
          </div>
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
