"use client";
import { AppSidebar } from "@/components/home-sidebar";
import { ChartAreaInteractive } from "@/components/chart-area-interactive";
import { DataTable } from "@/components/domains-table";
import { SectionCards } from "@/components/section-cards";
import SiteHeader from "@/components/site-header";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import Link from "next/link";
import { Checkbox } from "@/components/ui/checkbox";
import { useState } from "react";
import { Button } from "@/components/ui/button";

function BotBlockCheckbox({ value, label, checked, onChange }: {
  value: string;
  label: string;
  checked: boolean;
  onChange: (value: string, checked: boolean) => void;
}) {
  return (
    <label className="flex items-center gap-2 px-4 py-2 rounded-lg border font-medium transition-colors cursor-pointer select-none bg-muted/30 hover:bg-muted/50 border-muted-foreground/30 text-muted-foreground data-[state=checked]:border-blue-600 data-[state=checked]:bg-blue-600 data-[state=checked]:text-white">

      <span>{label}</span>
    </label>
  );
}

export default function NewDomainPage() {
  // Manage selection state for the three checkboxes
  const [blockModes, setBlockModes] = useState<{ [key: string]: boolean }>(() => {
    if (typeof window !== "undefined") {
      const saved = window.localStorage.getItem("aiBotBlockModes");
      if (saved) return JSON.parse(saved);
    }
    return { all: true, ads: false, off: false };
  });

  const handleBlockModeChange = (value: string, checked: boolean) => {
    setBlockModes((prev) => {
      const updated = { ...prev, [value]: checked };
      if (typeof window !== "undefined") {
        window.localStorage.setItem("aiBotBlockModes", JSON.stringify(updated));
      }
      return updated;
    });
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
        <SiteHeader title="New Domain" />
        <div className="flex flex-1 flex-col">
          <div className="@container/main flex flex-1 flex-col gap-2">
            <div className="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
              <div className="px-4 lg:px-6">
    {/* Domain input and DNS scan options */}
    <div className="grid w-full max-w-sm items-center gap-3">
      <Label htmlFor="domain-name">Domain Name</Label>
      <Input type="text" id="domain-name" placeholder="example.com" />
      <div className="flex flex-col gap-6 mt-4">
        <Label className="hover:bg-accent/50 flex items-start gap-3 rounded-lg border p-3 has-[[aria-checked=true]]:border-blue-600 has-[[aria-checked=true]]:bg-blue-50 dark:has-[[aria-checked=true]]:border-blue-900 dark:has-[[aria-checked=true]]:bg-blue-950">
          <Checkbox
            id="quick-scan"
            defaultChecked
            className="data-[state=checked]:border-blue-600 data-[state=checked]:bg-blue-600 data-[state=checked]:text-white dark:data-[state=checked]:border-blue-700 dark:data-[state=checked]:bg-blue-700"
          />
          <div className="grid gap-1.5 font-normal">
            <p className="text-sm leading-none font-medium">
              Automatically scan for DNS Records
            </p>
            <p className="text-muted-foreground text-sm">
              NetGoat will automatically scan for DNS records and add them to your domain.
            </p>
          </div>
        </Label>
        <div className="flex items-center gap-3">
          <Checkbox id="manual" />
          <Label htmlFor="manual">Manually input records</Label>
        </div>
        <div className="flex items-center gap-3">
          <Checkbox id="upload-zone" />
          <Label htmlFor="upload-zone">Upload a DNS zone file</Label>
        </div>
      </div>
    </div>

    {/* AI Crawler/Bot control section */}
    <div className="mt-8">
      <h1 className="text-lg my-4 font-semibold tracking-tight">
        Control how Bots or AI Crawlers interact with your site
      </h1>
      <div className="flex flex-col gap-4">
        <div className="flex gap-3">
          {/* AI bot block mode selection as checkboxes */}
          {[
            { label: "Block on all pages", value: "all" },
            { label: "Block only on hostnames with ads", value: "ads" },
            { label: "Do not block (off)", value: "off" },
          ].map((opt) => (
            <BotBlockCheckbox
              key={opt.value}
              value={opt.value}
              label={opt.label}
              checked={!!blockModes[opt.value]}
              onChange={handleBlockModeChange}
            />
          ))}
        </div>
        <div className="flex items-center gap-2 mt-2">
          <Checkbox id="manage-robots" defaultChecked />
          <Label htmlFor="manage-robots">Manage AI bot traffic with robots.txt</Label>
        </div>
      </div>
          <Button className="mt-3">
        Continue
    </Button>
    </div>


              </div>
            </div>
          </div>
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
