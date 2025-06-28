"use client"

import * as React from "react"
import {
  IconCamera,
  IconChartBar,
  IconDashboard,
  IconDatabase,
  IconFileAi,
  IconFileDescription,
  IconFileWord,
  IconRobot,
  IconFolder,
  IconHelp,
  IconInnerShadowTop,
  IconListDetails,
  IconReport,
  IconSearch,
  IconSettings,
  IconUsers,
  IconCloud,
  IconRobotFace,
  IconShieldBolt,
} from "@tabler/icons-react"

import {
  AudioWaveform,
  BookOpen,
  Bot,
  Command,
  Frame,
  GalleryVerticalEnd,
  Map,
  PieChart,
  Settings2,
  SquareTerminal,
} from "lucide-react"

import { usePathname } from "next/navigation"

import { NavDocuments } from "@/components/nav-documents"
import { NavMain } from "@/components/nav-main"
import { NavSecondary } from "@/components/nav-secondary"
import { NavUser } from "@/components/nav-user"
import { TeamSwitcher } from "@/components/team-switcher"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const pathname = usePathname()
const base = pathname.split("/")[2];

  let teams = [
    {
      name: "Cloudable",
      logo: "https://cdn.discordapp.com/icons/1350110102337749062/c50196ba4c1430be09812a5492546623.png",
      plan: "Enterprise"
    },
    {
      name: "Rewire",
      logo: "https://cdn.discordapp.com/avatars/845312519001342052/ad209fd629974989e27ce2ac4f51a97f.png?size=1024",
      plan: "Enterprise"
    },
  ]

  const data = {
    user: {
      name: "Ducky",
      email: "ducky@cloudable.dev",
      avatar: "https://cdn.discordapp.com/avatars/845312519001342052/ad209fd629974989e27ce2ac4f51a97f.png?size=1024",
    },
    navMain: [
      {
        title: "Dashboard",
        url: `${base}/dashboard`,
        icon: IconDashboard,
      },
      {
        title: "DNS",
        url: `${base}/dns`,
        icon: IconCloud,
      },
      {
        title: "Analytics",
        url: `${base}/analytics`,
        icon: IconChartBar,
      },
      {
        title: "WAF",
        url: `${base}/waf`,
        icon: IconShieldBolt,
      },
      {
        title: "Captcha",
        url: `${base}/captcha`,
        icon: IconRobotFace,
      },
      {
        title: "Error Tracking",
        url: `${base}/errors`,
        icon: IconReport,
      }
    ],
  }

  return (
    <Sidebar collapsible="offcanvas" {...props}>
      <SidebarHeader>
        <TeamSwitcher teams={teams} />
      </SidebarHeader>
      <SidebarContent>
        <NavMain items={data.navMain} />
      </SidebarContent>
      <SidebarFooter>
        <NavUser user={data.user} />
      </SidebarFooter>
    </Sidebar>
  )
}
