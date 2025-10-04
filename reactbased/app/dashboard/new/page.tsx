"use client";
import { useState } from "react";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { Globe, Shield, Zap, Upload, FileText, Wrench } from "lucide-react";

function BotBlockCard({
  value,
  label,
  checked,
  onChange,
}: {
  value: string;
  label: string;
  checked: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <div
      onClick={() => onChange(value)}
      className={cn(
        "group relative cursor-pointer select-none rounded-lg border-2 p-4 transition-all duration-200 hover:shadow-md",
        checked
          ? "border-primary shadow-sm ring-1 ring-primary/20"
          : "border-border bg-card hover:border-border/80 hover:bg-accent/50",
      )}
    >
      <div className="flex items-center justify-between">
        <p className="font-medium text-sm text-foreground">{label}</p>
        <div
          className={cn(
            "h-4 w-4 rounded-full border-2 transition-colors",
            checked
              ? "border-primary bg-primary"
              : "border-muted-foreground group-hover:border-foreground/60",
          )}
        >
          {checked && (
            <div className="h-full w-full rounded-full bg-primary-foreground scale-50" />
          )}
        </div>
      </div>
    </div>
  );
}

function DNSOrRev({
  value,
  label,
  checked,
  onChange,
}: {
  value: string;
  label: string;
  checked: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <div
      onClick={() => onChange(value)}
      className={cn(
        "group relative cursor-pointer select-none rounded-lg border-2 p-4 transition-all duration-200 hover:shadow-md",
        checked
          ? "border-primary shadow-sm ring-1 ring-primary/20"
          : "border-border bg-card hover:border-border/80 hover:bg-accent/50",
      )}
    >
      <div className="flex items-center justify-between">
        <p className="font-medium text-sm text-foreground">{label}</p>
        <div
          className={cn(
            "h-4 w-4 rounded-full border-2 transition-colors",
            checked
              ? "border-primary bg-primary"
              : "border-muted-foreground group-hover:border-foreground/60",
          )}
        >
          {checked && (
            <div className="h-full w-full rounded-full bg-primary-foreground scale-50" />
          )}
        </div>
      </div>
    </div>
  );
}

export default function NewDomainPage() {
  const [blockMode, setBlockMode] = useState<string>(() => {
    if (typeof window !== "undefined") {
      const saved = window.localStorage.getItem("aiBotBlockMode");
      if (saved) return saved;
    }
    return "all";
  });

  const [Mode, setMode] = useState<string>(() => {
    if (typeof window !== "undefined") {
      const saved = window.localStorage.getItem("modeChange");
      if (saved) return saved;
    }
    return "all";
  });

  const handleBlockChange = (value: string) => {
    setBlockMode(value);
    if (typeof window !== "undefined") {
      window.localStorage.setItem("aiBotBlockMode", value);
    }
  };

  const handleModeChange = (value: string) => {
    setMode(value);
    if (typeof window !== "undefined") {
      window.localStorage.setItem("modeChange", value);
    }
  };

  return (
    <div className="space-y-6 p-5">
      <div className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">
          Add New Domain
        </h1>
        <p className="text-muted-foreground">
          Configure your domain settings and DNS records
        </p>
      </div>

      <div className="space-y-6">
        {/* Domain Configuration Card */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Globe className="h-5 w-5 text-primary" />
              Domain Configuration
            </CardTitle>
            <CardDescription>
              Enter your domain name and choose how to configure DNS records
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="space-y-2">
              <Label htmlFor="domain-name" className="text-sm font-medium">
                Domain Name
              </Label>
              <Input
                id="domain-name"
                placeholder="example.com"
                className="h-11"
              />
            </div>

            <div className="space-y-4">
              <Label className="text-sm font-medium">
                DNS Configuration Method
              </Label>

              <Label className="group flex cursor-pointer items-start gap-4 rounded-lg border border-border p-4 transition-colors hover:bg-accent/50">
                <Checkbox
                  id="quick-scan"
                  defaultChecked
                  className="mt-0.5 data-[state=checked]:bg-primary data-[state=checked]:border-primary"
                />
                <div className="flex-1 space-y-1">
                  <div className="flex items-center gap-2">
                    <Zap className="h-4 w-4 text-primary" />
                    <span className="font-medium text-foreground">
                      Automatically scan for DNS Records
                    </span>
                    <Badge variant="secondary" className="text-xs">
                      Recommended
                    </Badge>
                  </div>
                  <p className="text-sm text-muted-foreground">
                    NetGoat will automatically scan for DNS records and add them
                    to your domain.
                  </p>
                </div>
              </Label>

              <Label className="group flex cursor-pointer items-center gap-4 rounded-lg border border-border p-4 transition-colors hover:bg-accent/50">
                <Checkbox
                  id="manual"
                  className="data-[state=checked]:bg-primary data-[state=checked]:border-primary"
                />
                <FileText className="h-4 w-4 text-muted-foreground" />
                <span className="font-medium text-foreground">
                  Manually input records
                </span>
              </Label>

              <Label className="group flex cursor-pointer items-center gap-4 rounded-lg border border-border p-4 transition-colors hover:bg-accent/50">
                <Checkbox
                  id="upload-zone"
                  className="data-[state=checked]:bg-primary data-[state=checked]:border-primary"
                />
                <Upload className="h-4 w-4 text-muted-foreground" />
                <span className="font-medium text-foreground">
                  Upload a DNS zone file
                </span>
              </Label>
            </div>
          </CardContent>
        </Card>

        {/* Mode Select */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Wrench className="h-5 w-5 text-primary" />
              DNS or Reverse Proxy
            </CardTitle>
            <CardDescription>
              Configure how AI bots and crawlers interact with your website
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="grid gap-3 sm:grid-cols-3">
              {[
                { label: "DNS Only", value: "DNSOnly" },
                { label: "Reverse proxy Only", value: "RevOnly" },
                {
                  label: "Both DNS And Reverse Proxy",
                  value: "BothDNSnReverse",
                },
              ].map((opt) => (
                <DNSOrRev
                  key={opt.value}
                  value={opt.value}
                  label={opt.label}
                  checked={Mode === opt.value}
                  onChange={handleModeChange}
                />
              ))}
            </div>
          </CardContent>
        </Card>

        {/* AI Bot Control Card */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Shield className="h-5 w-5 text-primary" />
              AI Bot & Crawler Control
            </CardTitle>
            <CardDescription>
              Configure how AI bots and crawlers interact with your website
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="grid gap-3 sm:grid-cols-3">
              {[
                { label: "Block on all pages", value: "all" },
                { label: "Block only on hostnames with ads", value: "ads" },
                { label: "Do not block (off)", value: "off" },
              ].map((opt) => (
                <BotBlockCard
                  key={opt.value}
                  value={opt.value}
                  label={opt.label}
                  checked={blockMode === opt.value}
                  onChange={handleBlockChange}
                />
              ))}
            </div>

            <Label className="flex cursor-pointer items-center gap-3 rounded-lg border border-border p-4 transition-colors hover:bg-accent/50">
              <Checkbox
                id="manage-robots"
                defaultChecked
                className="data-[state=checked]:bg-primary data-[state=checked]:border-primary"
              />
              <span className="font-medium text-foreground">
                Manage AI bot traffic with robots.txt
              </span>
            </Label>
          </CardContent>
        </Card>

        <div className="flex justify-start">
          <Button size="lg" className="min-w-32">
            Continue
          </Button>
        </div>
      </div>
    </div>
  );
}
