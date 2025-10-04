"use client";

import { cn } from "@/lib/utils";
import { Separator } from "@/components/ui/separator";

interface PageTitleProps {
  title: string;
  subtitle?: string;
  actions?: React.ReactNode;
}

export function PageTitle({ title, subtitle, actions }: PageTitleProps) {
  return (
    <div className="w-full  bg-background/70 backdrop-blur-sm sticky top-0 z-30">
      <div className="w-full mx-auto flex items-center justify-between">
        <div>
          <h1 className="text-2xl text-left font-semibold tracking-tight">
            {title}
          </h1>
          {subtitle && (
            <p className="text-sm text-left text-muted-foreground mt-1">
              {subtitle}
            </p>
          )}
        </div>
        {actions && <div className="flex items-center gap-2">{actions}</div>}
      </div>
      <Separator className="my-3" />
    </div>
  );
}
