"use client";

import * as React from "react";
import Image from "next/image";
import { ChevronsUpDown, Plus } from "lucide-react";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar";

export function TeamSwitcher({
  teams,
  onTeamChange,
}: {
  teams: {
    name: string;
    logo: string;
    plan: string;
  }[];
  onTeamChange?: (team: { name: string; logo: string; plan: string }) => void;
}) {
  const { isMobile } = useSidebar();
  const [activeTeam, setActiveTeam] = React.useState(teams[0]);
  // Track image error per team by name
  const [imgErrorMap, setImgErrorMap] = React.useState<{ [name: string]: boolean }>({});

  if (!activeTeam) {
    return null;
  }

  function handleSelect(team: { name: string; logo: string; plan: string }) {
    setActiveTeam(team);
    if (onTeamChange) onTeamChange(team);
  }

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton
              size="lg"
              className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
            >
              <div className="text-sidebar-primary-foreground flex aspect-square size-8 items-center justify-center rounded-lg">
                {imgErrorMap[activeTeam.name] ? (
                  <Image
                    src="https://cdnjs.cloudflare.com/ajax/libs/emoji-datasource-apple/15.1.2/img/apple/64/1f47b.png"
                    alt={activeTeam.name}
                    width={24}
                    height={24}
                    className="text-sidebar-primary-foreground flex aspect-square size-8 items-center justify-center rounded-lg"
                  />
                ) : (
                  <Image
                    src={activeTeam.logo}
                    alt={activeTeam.name}
                    width={24}
                    height={24}
                    className="text-sidebar-primary-foreground flex aspect-square size-8 items-center justify-center rounded-lg"
                    onError={() => setImgErrorMap((prev) => ({ ...prev, [activeTeam.name]: true }))}
                  />
                )}
              </div>
              <div className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-medium">{activeTeam.name}</span>
                <span className="truncate text-xs">{activeTeam.plan}</span>
              </div>
              <ChevronsUpDown className="ml-auto" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="w-(--radix-dropdown-menu-trigger-width) min-w-56 rounded-lg"
            align="start"
            side={isMobile ? "bottom" : "right"}
            sideOffset={4}
          >
            <DropdownMenuLabel className="text-muted-foreground text-xs">
              Teams
            </DropdownMenuLabel>
            {teams.map((team, index) => (
              <DropdownMenuItem
                key={team.name}
                onClick={() => handleSelect(team)}
                className="gap-2 p-2"
              >
                <div className="">
                  {imgErrorMap[team.name] ? (
                    <Image
                      src="https://cdnjs.cloudflare.com/ajax/libs/emoji-datasource-apple/15.1.2/img/apple/64/1f47b.png"
                      alt={team.name}
                      width={24}
                      height={24}
                      className="flex size-6 items-center justify-center rounded-md border"
                    />
                  ) : (
                    <Image
                      src={team.logo}
                      alt={team.name}
                      width={24}
                      height={24}
                      className="flex size-6 items-center justify-center rounded-md border"
                      onError={() => setImgErrorMap((prev) => ({ ...prev, [team.name]: true }))}
                    />
                  )}
                </div>
                {team.name}
                <DropdownMenuShortcut>⌘{index + 1}</DropdownMenuShortcut>
              </DropdownMenuItem>
            ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem className="gap-2 p-2">
              <div className="flex size-6 items-center justify-center rounded-md border bg-transparent">
                <Plus className="size-4" />
              </div>
              <div className="text-muted-foreground font-medium">Add team</div>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}
