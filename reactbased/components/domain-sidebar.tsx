"use client";
import { useParams } from "next/navigation";

import * as React from "react";
import {
  IconChartBar,
  IconDashboard,
  IconReport,
  IconCloud,
  IconRobotFace,
  IconShieldBolt,
  IconTicket,
  IconServer,
} from "@tabler/icons-react";

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
} from "lucide-react";

import { usePathname } from "next/navigation";

import { NavDocuments } from "@/components/nav-documents";
import { NavMain } from "@/components/nav-main";
import { NavSecondary } from "@/components/nav-secondary";
import { NavUser } from "@/components/nav-user";
import { TeamSwitcher } from "@/components/team-switcher";
import { useRouter } from "next/navigation";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";
import axios from "axios";
import { useEffect, useState } from "react";

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const params = useParams();
  // Get current domain slug from URL
  const currentDomain = params?.slug || ""; // adjust if your route uses something else

  type Domain = {
    group: string;
    name: string;
    status: "active" | "inactive" | "pending";
    lastSeen: string;
  };

  type Role = "user" | "admin";

  type Data = {
    username: string;
    email: string;
    role: Role[]; // array of roles
    domains: Domain[];
    _id: string;
    createdAt: string; // or Date if you're parsing it
    updatedAt: string; // or Date if you're parsing it
  };

  const [datas, setDatas] = useState<Data | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const sessionStr = localStorage.getItem("session");
    if (!sessionStr) {
      setLoaded(true);
      setError("No session found. Please log in again.");
      return;
    }

    let lsd: any = {};
    try {
      lsd = JSON.parse(sessionStr || "{}");
    } catch (e) {
      setLoaded(true);
      setError("Session data corrupted. Please log in again.");
      return;
    }

    const id = lsd.userId;
    if (!id) {
      setLoaded(true);
      setError("Session missing user ID. Please log in again.");
      return;
    }

    axios
      .get(`${process.env.backendapi}/api/${id}`, {
        headers: {
          Authorization: `Bearer ${localStorage.getItem("jwt")}`,
        },
      })
      .then((res) => {
        setDatas(res.data || null);
        localStorage.setItem("userFD", JSON.stringify(res.data));
        setLoaded(true);
        setError(null);
      })
      .catch((err) => {
        console.error("Domain fetch failed", err);
        setLoaded(true);
        setError("Failed to fetch domains. Please try again.");
      });
  }, []);

  const domains = datas?.domains || [];

  // Only show "No Domains" if loaded and truly empty, otherwise show domains
  let teams: { name: string; logo: string; plan: string }[] = [];
  if (loaded) {
    if (domains.length === 0) {
      teams = [
        {
          name: "No Domains",
          logo: "https://cdnjs.cloudflare.com/ajax/libs/emoji-datasource-apple/15.1.2/img/apple/64/1f47b.png",
          plan: "None",
        },
      ];
    } else {
      // Move the current domain to the front
      const mapped = domains.map((d) => ({
        name: d.name,
        logo: `https://t2.gstatic.com/faviconV2?client=SOCIAL&type=FAVICON&fallback_opts=TYPE,SIZE,URL&url=https://${d.name}&size=128`,
        plan: d.status
          ? d.status.charAt(0).toUpperCase() + d.status.slice(1)
          : "Active",
      }));
      const idx = mapped.findIndex(
        (t) => t.name.toLowerCase() === String(currentDomain).toLowerCase()
      );
      if (idx > 0) {
        // Move current domain to the front
        const [current] = mapped.splice(idx, 1);
        teams = [current, ...mapped];
      } else {
        teams = mapped;
      }
    }
  }

  // Use slug (currentDomain) to find the current domain for preview
  /*
  const currentTeam =
    teams.find(
      (t) =>
        t.name && t.name.toLowerCase() === String(currentDomain).toLowerCase()
    ) || teams[0];
   */
  const data = {
    user: {
      name: "ducky",
      email: "ducky@cloudable.dev",
      avatar: "",
    },
    navMain: [
      {
        title: "Dashboard",
        url: `/dashboard/${currentDomain}`,
        icon: IconDashboard,
      },
      {
        title: "DNS",
        url: `/dashboard/${currentDomain}/dns`,
        icon: IconCloud,
      },
      {
        title: "Proxies",
        url: `/dashboard/${currentDomain}/proxies`,
        icon: IconServer,
      },
      {
        title: "Certs",
        url: `/dashboard/${currentDomain}/certs`,
        icon: IconTicket,
      },
      {
        title: "Analytics",
        url: `/dashboard/${currentDomain}/analytics`,
        icon: IconChartBar,
      },
      {
        title: "WAF",
        url: `/dashboard/${currentDomain}/waf`,
        icon: IconShieldBolt,
      },
      {
        title: "Captcha",
        url: `/dashboard/${currentDomain}/captcha`,
        icon: IconRobotFace,
      },
      {
        title: "Error Tracking",
        url: `/dashboard/${currentDomain}/errors`,
        icon: IconReport,
      },
    ],
  };

  const router = useRouter();

  // Only render sidebar if loaded (prevents empty TeamSwitcher)
  if (!loaded) {
    return (
      <div className="flex items-center justify-center h-full w-full">
        <span className="text-muted-foreground">Loading domains...</span>
      </div>
    );
  }

  function handleTeamSwitch(selected: { name: string }) {
    if (!selected || !selected.name || selected.name === currentDomain) return;
    // Use window.location for hard navigation if router.push doesn't work
    try {
      router.push(`/dashboard/${selected.name}`);
    } catch (e) {
      window.location.href = `/dashboard/${selected.name}`;
    }
  }

  return (
    <Sidebar collapsible="offcanvas" {...props}>
      <SidebarHeader>
        <TeamSwitcher teams={teams} onTeamChange={handleTeamSwitch} />
      </SidebarHeader>
      <SidebarContent>
        <NavMain items={data.navMain} />
      </SidebarContent>
      <SidebarFooter>
        <NavUser user={data.user} />
      </SidebarFooter>
    </Sidebar>
  );
}
