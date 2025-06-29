"use client";

import React, { useState } from "react";
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
import { ShieldCheck, ShieldX, Clock, Download, RefreshCw, Trash2, Lock, Unlock, Award } from "lucide-react";

const mockCerts = [
  {
    id: 1,
    domain: "app.example.com",
    issuer: "Let's Encrypt",
    status: "valid",
    expiry: "2025-08-01T12:00:00Z",
    type: "DV",
    selfSigned: false,
  },
  {
    id: 2,
    domain: "api.example.com",
    issuer: "Let's Encrypt",
    status: "expiring",
    expiry: "2025-07-02T12:00:00Z",
    type: "DV",
    selfSigned: false,
  },
  {
    id: 3,
    domain: "cdn.example.com",
    issuer: "Self-Signed",
    status: "valid",
    expiry: "2026-01-01T00:00:00Z",
    type: "Self-Signed",
    selfSigned: true,
  },
  {
    id: 4,
    domain: "admin.example.com",
    issuer: "Let's Encrypt",
    status: "expired",
    expiry: "2024-06-01T12:00:00Z",
    type: "DV",
    selfSigned: false,
  },
];

const statusColors = {
  valid: "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300",
  expiring: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300",
  expired: "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300",
};

export default function CertsPage() {
  const [filter, setFilter] = useState("all");
  const filteredCerts =
    filter === "all"
      ? mockCerts
      : filter === "selfsigned"
      ? mockCerts.filter((c) => c.selfSigned)
      : mockCerts.filter((c) => c.status === filter);

  return (
    <SidebarProvider
      style={{
        "--sidebar-width": "calc(var(--spacing) * 72)",
        "--header-height": "calc(var(--spacing) * 12)",
      } as React.CSSProperties}
    >
      <AppSidebar variant="inset" />
      <SidebarInset>
        <SiteHeader title="Certificates" />
        <div className="flex flex-1 flex-col">
          <div className="@container/main flex flex-1 flex-col gap-2">
            <div className="flex flex-col gap-6 p-6 md:p-10">
              <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-4">
                <h2 className="text-2xl font-bold tracking-tight text-foreground">Certificates</h2>
                <Tabs defaultValue={filter} onValueChange={setFilter} className="w-fit">
                  <TabsList>
                    <TabsTrigger value="all">All</TabsTrigger>
                    <TabsTrigger value="valid">Valid</TabsTrigger>
                    <TabsTrigger value="expiring">Expiring</TabsTrigger>
                    <TabsTrigger value="expired">Expired</TabsTrigger>
                    <TabsTrigger value="selfsigned">Self-Signed</TabsTrigger>
                  </TabsList>
                </Tabs>
              </div>
              <motion.div layout className="overflow-x-auto rounded-lg border bg-background transition-all">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="pl-6 font-semibold text-foreground">Domain</TableHead>
                      <TableHead className="font-semibold text-foreground">Issuer</TableHead>
                      <TableHead className="font-semibold text-foreground">Status</TableHead>
                      <TableHead className="font-semibold text-foreground">Expiry</TableHead>
                      <TableHead className="font-semibold text-foreground">Type</TableHead>
                      <TableHead className="pr-6 font-semibold text-foreground text-right">Action</TableHead>
                    </TableRow>
                  </TableHeader>
                  <motion.tbody layout>
                    <AnimatePresence initial={false}>
                      {filteredCerts.map((cert) => (
                        <motion.tr
                          key={cert.id}
                          layout
                          initial={{ opacity: 0, y: 10 }}
                          animate={{ opacity: 1, y: 0 }}
                          exit={{ opacity: 0, y: -10 }}
                          transition={{ duration: 0.2 }}
                          className="border-b"
                        >
                          <TableCell className="font-mono font-medium flex items-center gap-2">
                            <Award className="w-4 h-4 text-muted-foreground" />
                            {cert.domain}
                          </TableCell>
                          <TableCell>{cert.issuer}</TableCell>
                          <TableCell>
                            <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold ${statusColors[cert.status as keyof typeof statusColors] || ''}`}>
                              {cert.status.charAt(0).toUpperCase() + cert.status.slice(1)}
                            </span>
                          </TableCell>
                          <TableCell>
                            {new Date(cert.expiry).toLocaleDateString()} {cert.status === 'expiring' && <span className="ml-2 text-yellow-600 dark:text-yellow-400">(Soon)</span>}
                            {cert.status === 'expired' && <span className="ml-2 text-red-600 dark:text-red-400">(Expired)</span>}
                          </TableCell>
                          <TableCell>
                            {cert.selfSigned ? (
                              <span className="inline-flex items-center gap-1 text-muted-foreground"><Unlock className="w-4 h-4" /> Self-Signed</span>
                            ) : (
                              <span className="inline-flex items-center gap-1 text-cyan-600 dark:text-cyan-400"><Lock className="w-4 h-4" /> {cert.type}</span>
                            )}
                          </TableCell>
                          <TableCell className="pr-6 text-right flex gap-2">
                            <DropdownMenu>
                              <DropdownMenuTrigger className="text-center ">
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
                                  {`${cert.domain}`}
                                </DropdownMenuLabel>
                                <DropdownMenuSeparator />
                                <DropdownMenuItem className="w-full">
                                  <RefreshCw className="w-full h-4 mr-1" />
                                  Renew
                                </DropdownMenuItem>
                                <DropdownMenuItem className="w-full">
                                  <Download className="w-full h-4 mr-1" />
                                  Download
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
