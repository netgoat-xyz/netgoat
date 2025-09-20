// eslint-disable-next-line @typescript-eslint/no-explicit-any
"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import Image from "next/image";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Select,
  SelectTrigger,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  Check,
  Database,
  User,
  Settings,
  SunMoon,
  Server,
  Globe,
  Shield,
  BarChart,
  Container,
  Clock,
  Cpu,
} from "lucide-react";

const steps = [
  { title: "Welcome", icon: Check },
  { title: "Database Setup", icon: Database },
  { title: "Admin Account", icon: User },
  { title: "Recommended Settings", icon: Settings },
  { title: "Theme Selector", icon: SunMoon },
  { title: "Mode", icon: Server },
  { title: "Domain Setup", icon: Globe },
  { title: "Privacy", icon: Shield },
  { title: "LogDB & Stat Server", icon: BarChart },
  { title: "Deployment Type", icon: Container },
  { title: "LogDB Retention", icon: Clock },
  { title: "Core Allocation", icon: Cpu },
  { title: "Hostnames / Nameservers", icon: Server },
];

export default function SetupPage() {
  const [step, setStep] = useState(0);
  const [db, setDb] = useState("MongoDB (Default)");
  const [direction, setDirection] = useState(0);
  const [smtpOn, setSmtpOn] = useState(false);
  const [limitsOn, setlimitsOn] = useState(false);
  const [theme, setTheme] = useState<"dark" | "light" | "system">("system");
  const opModeOptions = [
        {
      value: "DNSOnly",
      label: "DNS Only",
      desc: "Minimal & night-friendly",
      img: "/setup/darkmode.png",
    },
    {
      value: "DNSReverse Proxy",
      label: "DNS + Reverse Proxy",
      desc: "Bright & airy workspace",
      img: "/setup/lightmode.png",
    },
    {
      value: "ReverseProxy",
      label: "Reverse Proxy Only",
      desc: "Follow OS appearance",
      img: "/setup/system.png",
    },
  ];
  const themeOptions = [
    {
      value: "dark",
      label: "Dark",
      desc: "Minimal & night-friendly",
      img: "/setup/darkmode.png",
    },
    {
      value: "light",
      label: "Light",
      desc: "Bright & airy workspace",
      img: "/setup/lightmode.png",
    },
    {
      value: "system",
      label: "System",
      desc: "Follow OS appearance",
      img: "/setup/system.png",
    },
  ];
  const next = () => {
    setDirection(1);
    setStep((s) => Math.min(s + 1, steps.length - 1));
  };
  const back = () => {
    setDirection(-1);
    setStep((s) => Math.max(s - 0, 0));
  };
  const isLast = step === steps.length - 1;

  const StepContent = () => {
    switch (step) {
      case 0:
        return (
          <motion.div
            initial={{ opacity: 0, y: 40 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8 }}
            className="h-full flex max-w-xl flex-col"
          >
            <Image
              src="/banner.png"
              alt="NetGoat Logo"
              width={2829}
              height={800}
              className="mx-auto mb-6 drop-shadow-lg"
            />

            <h1 className="text-4xl font-extrabold tracking-tight text-white mb-3">
              Welcome to <span className="text-indigo-300">NetGoat</span>
            </h1>

            <p className="text-lg text-white/80 mb-8">
              Your new experience to networking.
              <br />
              Made to be simple, Made to be powerful.
              <br />
              <span
                className="hover:text-indigo-300 transition-all duration-300
  hover:[text-shadow:0_0_5px_rgba(129,140,248,0.8),0_0_10px_rgba(129,140,248,0.7),0_0_20px_rgba(129,140,248,0.6),0_0_40px_rgba(129,140,248,0.5),0_0_60px_rgba(129,140,248,0.4)]"
              >
                Made for you.
              </span>
            </p>

            <div className="mt-auto pt-8">
              <Button
                onClick={next}
                className="w-full px-8 py-6 text-lg font-semibold rounded-xl shadow-md hover:scale-105 transition"
              >
                Let&apos;s Start
              </Button>
            </div>
          </motion.div>
        );
      case 1:
        return (
          <div className="space-y-4">
            <h1 className="font-bold text-2xl">Step 1: Database Setup</h1>

            <Label>Database Type</Label>
            <Select value={db} onValueChange={setDb}>
              <SelectTrigger>{db}</SelectTrigger>
              <SelectContent>
                <SelectItem value="SQlite">SQLite</SelectItem>
                <SelectItem value="MySQL">MySQL / MariaDB</SelectItem>
                <SelectItem value="MongoDB">MongoDB</SelectItem>
                <SelectItem value="PostgresSQL">PostgreSQL</SelectItem>
              </SelectContent>
            </Select>

            <div className="flex items-center space-x-3">
              <div className="w-full">
                <Label className="mb-2">Database String</Label>
                <Input
                  className="w-full"
                  placeholder="mongodb://user:pass@localhost/netgoat"
                />
              </div>
              <div className="w-full">
                <Label className="mb-2">Database Name</Label>
                <Input className="w-full" placeholder="netgoat" />
              </div>
            </div>

            <div className="flex items-center space-x-3">
              <div className="w-full">
                <Label className="mb-2">Database Host</Label>
                <Input className="w-full" placeholder="localhost" />
              </div>

              <div className="w-full">
                <Label className="mb-2">Database Port</Label>
                <Input className="w-full" placeholder="3306" />
              </div>
            </div>

            <div className="flex items-center space-x-3">
              <div className="w-full">
                <Label className="mb-2">Database Username</Label>
                <Input className="w-full" placeholder="root" />
              </div>
              <div className="w-full">
                <Label className="mb-2">Database Password</Label>
                <Input
                  className="w-full"
                  type="password"
                  placeholder="••••••••"
                />
              </div>
            </div>
          </div>
        );
      case 2:
        return (
          <div className="space-y-4">
            <h1 className="font-bold text-2xl">Step 2: Admin Account Setup</h1>
            <div className="flex items-center space-x-3">
              <div className="w-full">
                <Label className="mb-2">Admin Username</Label>
                <Input className="w-full" placeholder="root" />
              </div>
              <div className="w-full">
                <Label className="mb-2">Password</Label>
                <Input
                  className="w-full"
                  type="password"
                  placeholder="••••••••"
                />
              </div>
            </div>{" "}
          </div>
        );
      case 3:
        return (
          <div className="space-y-4">
            <h1 className="font-bold text-2xl">Step 3: Recommended settings</h1>

            <Label className="hover:bg-accent/50 flex items-start gap-3 rounded-lg border p-3 has-[[aria-checked=true]]:border-indigo-600 has-[[aria-checked=true]]:bg-indigo-50 dark:has-[[aria-checked=true]]:border-indigo-900 dark:has-[[aria-checked=true]]:bg-indigo-950">
              <Checkbox
                id="toggle-2"
                defaultChecked
                className="data-[state=checked]:border-indigo-600 data-[state=checked]:bg-indigo-600 data-[state=checked]:text-white dark:data-[state=checked]:border-indigo-700 dark:data-[state=checked]:bg-indigo-700"
              />
              <div className="grid gap-1.5 font-normal">
                <p className="text-sm leading-none font-medium">
                  Enable registrations
                </p>
                <p className="text-muted-foreground text-sm">
                  You can enable or disable registration at any time in admin
                  settings.
                </p>
              </div>
            </Label>
            <Label
              className="hover:bg-accent/50 flex items-start gap-3 rounded-lg border p-3
        has-[[aria-checked=true]]:border-indigo-600 has-[[aria-checked=true]]:bg-indigo-50
        dark:has-[[aria-checked=true]]:border-indigo-900 dark:has-[[aria-checked=true]]:bg-indigo-950"
            >
              <Checkbox
                id="toggle-2"
                checked={smtpOn}
                onCheckedChange={(val) => setSmtpOn(Boolean(val))}
                className="data-[state=checked]:border-indigo-600 data-[state=checked]:bg-indigo-600
          data-[state=checked]:text-white dark:data-[state=checked]:border-indigo-700
          dark:data-[state=checked]:bg-indigo-700"
              />
              <div className="grid gap-1.5 font-normal">
                <p className="text-sm leading-none font-medium">Enable SMTP</p>
                <p className="text-muted-foreground text-sm">
                  You can enable or disable SMTP Notifications at any time in
                  admin settings.
                </p>
              </div>
            </Label>

            <div className="grid gap-3">
              <div className="flex items-center space-x-3">
                <div className="w-full">
                  <Label className="mb-2">SMTP Host</Label>
                  <Input placeholder="SMTP Host" disabled={!smtpOn} />
                </div>
                <div className="w-full">
                  <Label className="mb-2">SMTP Port</Label>
                  <Input placeholder="SMTP Port" disabled={!smtpOn} />
                </div>
              </div>
              <div className="flex items-center space-x-3">
                <div className="w-full">
                  {" "}
                  <Label className="mb-2">Username</Label>
                  <Input placeholder="Username" disabled={!smtpOn} />
                </div>
                <div className="w-full">
                  <Label className="mb-2">Password</Label>
                  <Input
                    placeholder="Password"
                    type="password"
                    disabled={!smtpOn}
                  />
                </div>
              </div>
              <div>
                <h3 className="mt-2">
                  <Label className="text-lg">
                    {" "}
                    Any limits? The skies the limit!
                  </Label>
                  <Label
                    className="mt-2 hover:bg-accent/50 flex items-start gap-3 rounded-lg border p-3
        has-[[aria-checked=true]]:border-indigo-600 has-[[aria-checked=true]]:bg-indigo-50
        dark:has-[[aria-checked=true]]:border-indigo-900 dark:has-[[aria-checked=true]]:bg-indigo-950"
                  >
                    <Checkbox
                      id="toggle-2"
                      checked={limitsOn}
                      onCheckedChange={(val) => setlimitsOn(Boolean(val))}
                      className="data-[state=checked]:border-indigo-600 data-[state=checked]:bg-indigo-600
          data-[state=checked]:text-white dark:data-[state=checked]:border-indigo-700
          dark:data-[state=checked]:bg-indigo-700"
                    />
                    <div className="grid gap-1.5 font-normal">
                      <p className="text-sm leading-none font-medium">
                        Enable Limits
                      </p>
                      <p className="text-muted-foreground text-sm">
                        You can enable or disable Usage Limits at any time in
                        admin settings.
                      </p>
                    </div>
                  </Label>

                  <div className="flex items-center mt-3 space-x-3">
                    <div className="w-full">
                      <Label className="mb-2">DNS Record Limits</Label>
                      <Input
                        placeholder="(No limits!)"
                        type="number"
                        disabled={!limitsOn}
                      />
                    </div>
                    <div className="w-full">
                      <Label className="mb-2">WAF Rules</Label>
                      <Input
                        placeholder="(No limits!)"
                        type="number"
                        disabled={!limitsOn}
                      />
                    </div>
                  </div>
                  <div className="flex items-center mt-4 space-x-3">
                    <div className="w-full">
                      <Label className="mb-2">SSL Cert Limits</Label>
                      <Input
                        placeholder="(No limits!)"
                        type="number"
                        disabled={!limitsOn}
                      />
                    </div>
                    <div className="w-full">
                      <Label className="mb-2">Request Per Month</Label>
                      <Input
                        placeholder="(No limits!)"
                        type="number"
                        disabled={!limitsOn}
                      />
                    </div>
                  </div>
                </h3>
              </div>
            </div>
          </div>
        );
      case 4:
        return (
          <div className="space-y-4">
            <h1 className="font-bold text-2xl">Step 4: Personalization ✨</h1>

            <h2 className="text-white/90 font-medium">Select your Theme</h2>

            <div className="grid gap-6 sm:grid-cols-3">
              {themeOptions.map((opt) => (
                <button
                  key={opt.value}
onClick={() => setTheme(opt.value as "dark" | "light" | "system")}
                  className={`group flex  flex-col rounded-2xl overflow-hidden ring-1 ring-white/20 transition
              ${
                theme === opt.value
                  ? "ring-2 ring-indigo-600"
                  : "hover:ring-indigo-300/60"
              }
            `}
                >
                  <div className="relative aspect-video w-full">
                    <Image
                      src={opt.img}
                      alt={`${opt.label} preview`}
                      fill
                      className="object-cover group-hover:scale-105 transition-transform duration-300"
                    />
                  </div>
                  <div className="p-4 text-left">
                    <p className="text-lg font-semibold text-white">
                      {opt.label}
                    </p>
                    <p className="text-sm text-white/70">{opt.desc}</p>
                  </div>
                </button>
              ))}
            </div>
          </div>
        );
      case 5:
        return (
          <div className="space-y-4">
            <h1 className="font-bold text-2xl">Step 5: Operating Mode</h1>

            <h2 className="text-white/90 font-medium">Select your Theme</h2>

            <div className="grid gap-6 sm:grid-cols-3">
              {opModeOptions.map((opt) => (
                <button
                  key={opt.value}
onClick={() => setTheme(opt.value as "dark" | "light" | "system")}                  className={`group flex flex-col rounded-2xl overflow-hidden ring-1 ring-white/20 transition
              ${
                theme === opt.value
                  ? "ring-2 ring-indigo-400"
                  : "hover:ring-indigo-300/60"
              }
            `}
                >
                  <div className="relative aspect-video w-full">
                    <Image
                      src={opt.img}
                      alt={`${opt.label} preview`}
                      fill
                      className="object-cover group-hover:scale-105 transition-transform duration-300"
                    />
                  </div>
                  <div className="p-4 text-left">
                    <p className="text-lg font-semibold text-white">
                      {opt.label}
                    </p>
                    <p className="text-sm text-white/70">{opt.desc}</p>
                  </div>
                </button>
              ))}
            </div>
          </div>
        );
      default:
        return <p>Step content here…</p>;
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-6">
      <Card className="w-full max-w-5xl flex flex-col md:flex-row bg-card text-card-foreground border-border shadow-lg">
        {/* Left step tracker */}
        <div className="md:w-72 border-r border-border p-4 flex flex-col gap-2">
          {steps.map((s, i) => {
            const Icon = s.icon;
            const active = i === step;
            return (
              <div
                key={i}
                className={`flex items-center gap-3 p-2 rounded-lg cursor-pointer transition ${
                  active
                    ? "bg-accent text-accent-foreground font-semibold"
                    : "text-muted-foreground"
                }`}
                onClick={() => {
                  setDirection(i > step ? 1 : -1);
                  setStep(i);
                }}
              >
                <Icon className="w-5 h-5" />
                <span>{s.title}</span>
              </div>
            );
          })}
        </div>

        {/* Right step content */}
        <div className="flex-1 p-6">
          <AnimatePresence initial={false} custom={direction}>
            <motion.div
              key={step}
              custom={direction}
              initial={{ x: direction > 0 ? 50 : -50, opacity: 0 }}
              animate={{ x: 0, opacity: 1 }}
              exit={{ x: direction > 0 ? -50 : 50, opacity: 0 }}
              transition={{ duration: 0.25 }}
            >
              <CardContent className="space-y-6">
                <StepContent />
                {step > 0 && (
                  <div className="flex justify-between pt-4">
                    <Button variant="secondary" onClick={back}>
                      Back
                    </Button>
                    <Button onClick={next}>{isLast ? "Finish" : "Next"}</Button>
                  </div>
                )}
              </CardContent>
            </motion.div>
          </AnimatePresence>
        </div>
      </Card>
    </div>
  );
}
