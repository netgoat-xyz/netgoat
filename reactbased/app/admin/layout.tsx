import { Metadata } from "next";
import { ReactNode } from "react";
import DashboardClientWrapper from "./layoutClient";

interface LayoutProps {
  children: ReactNode;
}

export default function DashboardLayout({ children }: LayoutProps) {
  return <DashboardClientWrapper>{children}</DashboardClientWrapper>;
}
