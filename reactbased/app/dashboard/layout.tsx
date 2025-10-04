"use client";

import { ReactNode } from "react";
import { Toaster } from "@/components/ui/sonner";

export default function DashboardLayout({ children }: { children: ReactNode }) {
  return (
    <div>
      {children}
      <Toaster />
    </div>
  );
}
