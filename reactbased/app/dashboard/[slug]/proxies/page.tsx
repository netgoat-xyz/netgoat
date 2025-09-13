"use client";

import React, { useState, useEffect } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import {
  Table,
  TableHead,
  TableRow,
  TableCell,
  TableHeader,
  TableBody,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { motion, AnimatePresence } from "framer-motion";
import { Globe, Edit, Trash2, RefreshCw, Lock, Unlock } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton"
import { Toaster } from "@/components/ui/sonar";
import { toast } from "sonner";

export default function Page({ params }: { params: { slug: string } }) {
  const [filter, setFilter] = useState("all");
  const [proxies, setProxies] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ slug: "", target: "", port: 80, ssl: false });

  const fetchProxies = async () => {
    setLoading(true);
    try {
      const res = await fetch(`${process.env.backendapi}/api/domains/${params.slug}`, {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${localStorage.getItem("jwt")}`,
        },
      });
      if (!res.ok) return setProxies([]);
      const data = await res.json();
      setProxies(data.proxied || []);
    } catch {
      setProxies([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchProxies();
  }, [params.slug]);

  const handleSubmit = async () => {
    if (!form.slug || !form.target) {
      toast.error("Slug and Target are required.");
      return;
    }
    setSaving(true);
    try {
      await fetch(`${process.env.backendapi}/api/manage-proxy?domain=${params.slug}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          slug: form.slug,
          domain: params.slug,
          ip: form.target,
          port: form.port,
          SSL: form.ssl,
        }),
      });
      setForm({ slug: "", target: "", port: 80, ssl: false });
      setModalOpen(false);
      fetchProxies();
      toast.success("Proxy created successfully!");
    } catch {
      toast.error("Failed to create proxy.");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (slug: string) => {
    toast(`Deleting ${slug}...`);
    await new Promise((r) => setTimeout(r, 500));
    setProxies((prev) => prev.filter((p) => p.slug !== slug));
    toast.success(`${slug} removed.`);
  };

  const filteredProxies =
    filter === "all" ? proxies : proxies.filter((p) => p.status === filter);

  const statusColors = {
    active: "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300",
    down: "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300",
    error: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300",
  };

  return (
    <>
      <div className="flex flex-1 flex-col p-6 md:p-10 gap-6">
        {/* Sticky header */}
        <div className="sticky top-0 bg-background z-10 flex flex-col sm:flex-row sm:items-center sm:justify-between pb-2">
          <h2 className="text-2xl font-semibold tracking-tight text-foreground">
            Reverse Proxies
          </h2>

          <Dialog open={modalOpen} onOpenChange={setModalOpen}>
            <DialogTrigger asChild>
              <Button>Add Reverse Proxy</Button>
            </DialogTrigger>
            <DialogContent className="max-w-2xl">
              <DialogHeader>
                <DialogTitle>Add Reverse Proxy</DialogTitle>
              </DialogHeader>

              <div className="space-y-4 mt-4">
                <div className="flex gap-4">
                  <div className="flex-1">
                    <Label>Slug</Label>
                    <Input
                      placeholder="e.g. app"
                      value={form.slug}
                      onChange={(e) => setForm({ ...form, slug: e.target.value })}
                    />
                  </div>
                  <div className="flex-1">
                    <Label>Target</Label>
                    <Input
                      placeholder="e.g. 192.168.1.10"
                      value={form.target}
                      onChange={(e) => setForm({ ...form, target: e.target.value })}
                    />
                  </div>
                </div>
                <div className="flex gap-4 items-center">
                  <div className="flex-1">
                    <Label>Port</Label>
                    <Input
                      type="number"
                      value={form.port}
                      onChange={(e) => setForm({ ...form, port: Number(e.target.value) })}
                    />
                  </div>
                  <div className="flex items-center gap-2">
                    <Label>SSL</Label>
                    <Switch
                      checked={form.ssl}
                      onCheckedChange={(checked) => setForm({ ...form, ssl: checked })}
                    />
                  </div>
                </div>
              </div>

              <div className="flex justify-end mt-6">
                <Button onClick={handleSubmit} disabled={saving}>
                  {saving && <RefreshCw className="w-4 h-4 animate-spin mr-2" />}
                  Save
                </Button>
              </div>
            </DialogContent>
          </Dialog>
        </div>

        {/* Filters */}
        <Tabs defaultValue={filter} onValueChange={setFilter} className="w-fit">
          <TabsList>
            <TabsTrigger value="all">All</TabsTrigger>
            <TabsTrigger value="active">Active</TabsTrigger>
            <TabsTrigger value="down">Down</TabsTrigger>
            <TabsTrigger value="error">Error</TabsTrigger>
          </TabsList>
        </Tabs>

        {/* Table / Empty / Loading */}
        <motion.div
          layout
          transition={{ layout: { duration: 0.35, type: "spring", bounce: 0.12 } }}
          className="overflow-x-auto rounded-lg border bg-background"
        >
          {loading ? (
            <div className="p-6 space-y-3">
              {[...Array(3)].map((_, i) => (
                <Skeleton key={i} className="h-10 w-full rounded" />
              ))}
            </div>
          ) : filteredProxies.length === 0 ? (
            <div className="flex flex-col items-center justify-center p-12 text-center space-y-3">
              <Globe className="w-10 h-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">No reverse proxies found.</p>
              <Button onClick={() => setModalOpen(true)}>Add Proxy</Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-6">Name</TableHead>
                  <TableHead>Target</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>SSL</TableHead>
                  <TableHead>Last Check</TableHead>
                  <TableHead className="pr-6 text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <AnimatePresence initial={false}>
                  {filteredProxies.map((proxy) => (
                    <motion.tr
                      key={proxy.slug}
                      layout
                      initial={{ opacity: 0, y: 10 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -10 }}
                      transition={{ duration: 0.2 }}
                      className="border-b"
                    >
                      <TableCell className="pl-6">{proxy.slug}</TableCell>
                      <TableCell className="font-mono">{proxy.ip}</TableCell>
                      <TableCell>
                        <span
                          className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold ${
                            statusColors[proxy.status as keyof typeof statusColors] ||
                            "bg-gray-100 text-gray-800"
                          }`}
                        >
                          {proxy.status || "unknown"}
                        </span>
                      </TableCell>
                      <TableCell>
                        {proxy.SSL ? (
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
                        {proxy.lastCheck
                          ? new Date(proxy.lastCheck).toLocaleString()
                          : "-"}
                      </TableCell>
                      <TableCell className="pr-6 text-right flex gap-2 justify-end">
                        <Button size="icon" variant="ghost">
                          <Edit className="w-4 h-4" />
                        </Button>
                        <Button size="icon" variant="ghost">
                          <RefreshCw className="w-4 h-4" />
                        </Button>
                        <Button
                          size="icon"
                          variant="ghost"
                          onClick={() => handleDelete(proxy.slug)}
                        >
                          <Trash2 className="w-4 h-4 text-red-500" />
                        </Button>
                      </TableCell>
                    </motion.tr>
                  ))}
                </AnimatePresence>
              </TableBody>
            </Table>
          )}
        </motion.div>
      </div>
    </>
  );
}
