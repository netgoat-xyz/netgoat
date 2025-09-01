// app/dashboard/DashboardClientWrapper.tsx
"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";

interface Props {
  children: React.ReactNode;
  params?: { slug?: string; section?: string };
}

export default function DashboardClientWrapper({ children, params }: Props) {
  const router = useRouter();
  const pathname = usePathname();

  const [ready, setReady] = useState(false);
  const [title, setTitle] = useState("Dashboard");

  // Update page title dynamically
  useEffect(() => {
    const parts = pathname.split("/").filter(Boolean);
    const domain = parts[1] ?? "Dashboard";
    const section = parts[2] ?? "Overview";
    const formattedSection = section.charAt(0).toUpperCase() + section.slice(1);
    setTitle(`${domain} | ${formattedSection}`);
    document.title = `${domain} | ${formattedSection}`;
  }, [pathname]);

  // Session check
  useEffect(() => {
    const jwt = localStorage.getItem("jwt");
    if (!jwt) {
      router.replace("/auth");
      return;
    }

    fetch("/api/session?session=" + jwt)
      .then((res) => res.json())
      .then((data) => {
        localStorage.setItem("session", JSON.stringify(data));
        setReady(true);
      })
      .catch(() => router.replace("/auth"));
  }, [router]);

  if (!ready) {
    return (
      <div className="flex h-screen items-center justify-center text-muted-foreground">
        Loading dashboard...
      </div>
    );
  }

  return <>{children}</>;
}
