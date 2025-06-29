"use client";

import React, { useState } from "react";
import { CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableHead,
  TableRow,
  TableCell,
  TableBody,
  TableHeader,
} from "@/components/ui/table";
import { motion, AnimatePresence } from "framer-motion";
import { SidebarProvider, SidebarInset } from "@/components/ui/sidebar";
import { AppSidebar } from "@/components/domain-sidebar";
import SiteHeader from "@/components/site-header";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";

import {
  Globe,
  Edit,
  Trash2,
  RefreshCw,
  TrendingUp,
  TrendingDown,
  Minus,
  Unlock,
  Lock,
} from "lucide-react";

const mockReverseProxies = [
  {
    id: 1,
    domain: "app.example.com",
    target: "10.0.0.2:3000",
    status: "active",
    ssl: true,
    lastCheck: "2025-06-30T10:00:00Z",
  },
  {
    id: 2,
    domain: "api.example.com",
    target: "10.0.0.3:8080",
    status: "down",
    ssl: false,
    lastCheck: "2025-06-30T09:45:00Z",
  },
  {
    id: 3,
    domain: "cdn.example.com",
    target: "10.0.0.4:80",
    status: "active",
    ssl: true,
    lastCheck: "2025-06-30T09:50:00Z",
  },
  {
    id: 4,
    domain: "admin.example.com",
    target: "10.0.0.5:9000",
    status: "error",
    ssl: false,
    lastCheck: "2025-06-30T09:55:00Z",
  },
];

const statusColors = {
  active:
    "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300",
  down: "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300",
  error:
    "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300",
};

export default function Page({ params }: { params: { domain: string; slug: string } }) {
  const [filter, setFilter] = useState("all");
  const filteredProxies =
    filter === "all"
      ? mockReverseProxies
      : mockReverseProxies.filter((p) => p.status === filter);

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
            <div className="flex flex-col gap-6 p-6 md:p-10">
              <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-4">
                <h2 className="text-2xl font-bold tracking-tight text-foreground">
                  Reverse Proxy List
                </h2>
                <Tabs
                  defaultValue={filter}
                  onValueChange={setFilter}
                  className="w-fit"
                >
                  <TabsList>
                    <TabsTrigger value="all">All</TabsTrigger>
                    <TabsTrigger value="active">Active</TabsTrigger>
                    <TabsTrigger value="down">Down</TabsTrigger>
                    <TabsTrigger value="error">Error</TabsTrigger>
                  </TabsList>
                </Tabs>
              </div>
              <motion.div
                layout
                layoutRoot
                transition={{
                  layout: { duration: 0.35, type: "spring", bounce: 0.12 },
                }}
                className="overflow-x-auto rounded-lg border bg-background transition-all"
              >
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="pl-6 font-semibold text-foreground">
                        Domain
                      </TableHead>
                      <TableHead className="font-semibold text-foreground">
                        Target
                      </TableHead>
                      <TableHead className="font-semibold text-foreground">
                        Status
                      </TableHead>
                      <TableHead className="font-semibold text-foreground">
                        SSL
                      </TableHead>
                      <TableHead className="font-semibold text-foreground">
                        Last Check
                      </TableHead>
                      <TableHead className="pr-6 font-semibold text-foreground text-right">
                        Action
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <motion.tbody layout>
                    <AnimatePresence initial={false}>
                      {filteredProxies.map((proxy) => (
                        <motion.tr
                          key={proxy.id}
                          layout
                          initial={{ opacity: 0, y: 10 }}
                          animate={{ opacity: 1, y: 0 }}
                          exit={{ opacity: 0, y: -10 }}
                          transition={{ duration: 0.2 }}
                          className="border-b"
                        >
                          <TableCell className="font-mono font-medium flex items-center gap-2">
                            <Globe className="w-4 h-4 text-muted-foreground" />
                            {proxy.domain}
                          </TableCell>
                          <TableCell className="font-mono">
                            {proxy.target}
                          </TableCell>
                          <TableCell>
                            <span
                              className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold ${
                                statusColors[
                                  proxy.status as keyof typeof statusColors
                                ]
                              }`}
                            >
                              {proxy.status.charAt(0).toUpperCase() +
                                proxy.status.slice(1)}
                            </span>
                          </TableCell>
                          <TableCell>
                            {proxy.ssl ? (
                              <span className="inline-flex items-center gap-1 text-cyan-600 dark:text-cyan-400">
                                <Lock className="w-4 h-4" /> Enabled
                              </span>
                            ) : (
                              <span className="inline-flex items-center gap-1 text-muted-foreground">
                                <Unlock className="w-4 h-4" /> Disabled
                              </span>
                            )}
                          </TableCell>
                          <TableCell>
                            {new Date(proxy.lastCheck).toLocaleString()}
                          </TableCell>
                          <TableCell className="text-right pr-6 text-sm flex justify-end text-muted-foreground">
                            <DropdownMenu>
                              <DropdownMenuTrigger className="">
                                <svg
                                  xmlns="http://www.w3.org/2000/svg"
                                  fill="none"
                                  viewBox="0 0 24 24"
                                  strokeWidth={1.5}
                                  stroke="currentColor"
                                  className="size-6"
                                >
                                  <path
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    d="M6.75 12a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0ZM12.75 12a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0ZM18.75 12a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Z"
                                  />
                                </svg>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent className="opacity-60 filter backdrop-blur-md">
                                <DropdownMenuLabel className="font-semibold text-foreground">
                                  {`${proxy.domain}`}
                                </DropdownMenuLabel>
                                <DropdownMenuSeparator />
                                <DropdownMenuItem className="w-full">
                                  <Edit className="w-full h-4 mr-1" />
                                  Edit
                                </DropdownMenuItem>
                                <DropdownMenuItem className="w-full">
                                  <RefreshCw className="w-full h-4 mr-1" />
                                  Test
                                </DropdownMenuItem>
                                <DropdownMenuItem className="w-full">
                                  <Trash2 className="w-full h-4 mr-1" />
                                  Delete
                                </DropdownMenuItem>
                              </DropdownMenuContent>
                            </DropdownMenu>
                          </TableCell>
                        </motion.tr>
                      ))}
                    </AnimatePresence>
                  </motion.tbody>
                </Table>
              </motion.div>
            </div>
          </div>
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
