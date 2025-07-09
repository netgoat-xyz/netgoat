// app/dashboard/layout.tsx
"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();

  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined") return;

    // Only check session for dashboard routes
    const jwt = localStorage.getItem("jwt");
    if (!jwt) {
      router.replace("/auth");
      return;
    }

    const session = localStorage.getItem("session");
      fetch("/api/session?session=" + jwt)
        .then((res) => res.json())
        .then((data) => {
          localStorage.setItem("session", JSON.stringify(data));
          setReady(true);
        })
        .catch((err) => {
          console.error("Session fetch failed", err);
          router.replace("/auth");
        });
  }, [pathname, router]);

  if (!ready) {
    return (
      <div className="flex h-screen items-center justify-center text-muted-foreground">
        Loading dashboard...
      </div>
    );
  }

  return <>{children}</>;
}
1