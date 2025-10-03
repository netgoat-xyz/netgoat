import { ReactNode } from "react"
import DashboardClientWrapper from "./layoutClient"
import { Toaster } from "@/components/ui/sonner"

export default function DashboardLayout({ children }: { children: ReactNode }) {
  return (
    <DashboardClientWrapper>
      {children}
      <Toaster />
    </DashboardClientWrapper>
  )
}
