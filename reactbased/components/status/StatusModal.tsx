"use client";

import { useEffect, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  CheckCircle2,
  AlertCircle,
  XCircle,
  Wrench,
  Clock,
  TrendingUp,
  Calendar,
} from "lucide-react";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface StatusModalProps {
  isOpen: boolean;
  onClose: () => void;
  service: {
    name: string;
    status: "operational" | "degraded" | "outage" | "maintenance";
    uptime: string;
    responseTime: string;
    description: string;
  };
}

interface HealthPing {
  timestamp: string;
  status: "operational" | "degraded" | "outage";
}

interface UptimeDay {
  date: string;
  status: "operational" | "degraded" | "outage";
}

const statusConfig = {
  operational: {
    icon: CheckCircle2,
    label: "Operational",
    color: "text-green-400",
    bgColor: "bg-green-900/40",
    borderColor: "border-green-700",
  },
  degraded: {
    icon: AlertCircle,
    label: "Degraded Performance",
    color: "text-yellow-400",
    bgColor: "bg-yellow-900/40",
    borderColor: "border-yellow-700",
  },
  outage: {
    icon: XCircle,
    label: "Service Outage",
    color: "text-red-400",
    bgColor: "bg-red-900/40",
    borderColor: "border-red-700",
  },
  maintenance: {
    icon: Wrench,
    label: "Under Maintenance",
    color: "text-blue-400",
    bgColor: "bg-blue-900/40",
    borderColor: "border-blue-700",
  },
};

export const StatusModal = ({ isOpen, onClose, service }: StatusModalProps) => {
  const config = statusConfig[service.status];
  const Icon = config.icon;
  const [uptimeHistory, setUptimeHistory] = useState<UptimeDay[]>([]);

  useEffect(() => {
    if (!isOpen) return;

    const fetchHistory = async () => {
      try {
        const res = await fetch("http://localhost:1933/api/health-pings");
        const data: HealthPing[] = await res.json();

        // Group pings by day
        const grouped: Record<string, string[]> = {};
        data.forEach((ping) => {
          const day = new Date(ping.timestamp).toISOString().split("T")[0];
          if (!grouped[day]) grouped[day] = [];
          grouped[day].push(ping.status);
        });

        // Compute daily status: worst ping of the day
        const days: UptimeDay[] = [];
        const today = new Date();
        for (let i = 89; i >= 0; i--) {
          const d = new Date();
          d.setDate(today.getDate() - i);
          const dayStr = d.toISOString().split("T")[0];
          const pings = grouped[dayStr] || ["operational"];
          let status: "operational" | "degraded" | "outage" = "operational";
          if (pings.includes("outage")) status = "outage";
          else if (pings.includes("degraded")) status = "degraded";
          days.push({
            date: d.toLocaleDateString("en-US", {
              month: "short",
              day: "numeric",
            }),
            status,
          });
        }

        setUptimeHistory(days);
      } catch (err) {
        console.error(err);
      }
    };

    fetchHistory();
  }, [isOpen]);

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl p-6 rounded-3xl bg-white/5 backdrop-blur-2xl border border-white/10 shadow-lg shadow-black/20 animate-scale-in transform transition-all duration-300 ease-out">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-3 text-2xl">
            <div
              className={`${config.bgColor} ${config.borderColor} border-2 p-2 rounded-lg`}
            >
              <Icon className={`w-6 h-6 ${config.color}`} />
            </div>
            {service.name}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-6 mt-4">
          <div
            className={`${config.bgColor} ${config.borderColor} border-2 rounded-xl p-4`}
          >
            <div
              className={`inline-flex items-center px-3 py-1 rounded-full ${config.bgColor} backdrop-blur-xl border ${config.borderColor} mb-6 relative overflow-hidden`}
            >
              <div className="absolute inset-0 bg-gradient-to-r from-white/5 via-white/10 to-white/5" />
              <div className="absolute top-0 left-0 right-0 h-px bg-gradient-to-r from-transparent via-white/30 to-transparent" />
              <span
                className={`text-xs font-light relative z-10 ${config.color}`}
              >
                {config.label}
              </span>
            </div>
            <p className="text-sm text-muted-foreground">
              {service.description}
            </p>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="bg-white/5 backdrop-blur-2xl border border-white/10 rounded-xl p-4 flex flex-col">
              <div className="flex items-center gap-2 mb-2">
                <TrendingUp className="w-4 h-4 text-primary" />
                <p className="text-sm font-medium text-muted-foreground">
                  Uptime
                </p>
              </div>
              <p className="text-2xl font-bold">{service.uptime}</p>
            </div>
            <div className="bg-white/5 backdrop-blur-2xl border border-white/10 rounded-xl p-4 flex flex-col">
              <div className="flex items-center gap-2 mb-2">
                <Clock className="w-4 h-4 text-primary" />
                <p className="text-sm font-medium text-muted-foreground">
                  Response Time
                </p>
              </div>
              <p className="text-2xl font-bold">{service.responseTime}</p>
            </div>
          </div>

          {/* Uptime History */}
          <div className="bg-white/5 backdrop-blur-2xl border border-white/10 rounded-2xl p-5 relative overflow-hidden">
            <div className="absolute inset-0 bg-gradient-to-r from-white/5 via-white/10 to-white/5 pointer-events-none rounded-2xl" />
            <div className="flex items-center gap-2 mb-4 z-10 relative">
              <Calendar className="w-4 h-4 text-primary" />
              <h4 className="font-semibold text-muted-foreground">
                90-Day Uptime History
              </h4>
            </div>

            <TooltipProvider>
              <div className="flex flex-wrap gap-1.5 z-10 relative">
                {uptimeHistory.map((day, index) => {
                  const blobColor =
                    day.status === "operational"
                      ? "bg-green-500"
                      : day.status === "degraded"
                        ? "bg-yellow-400"
                        : "bg-red-500";

                  const label =
                    day.status === "operational"
                      ? "Operational"
                      : day.status === "degraded"
                        ? "Degraded"
                        : "Outage";

                  return (
                    <Tooltip key={index}>
                      <TooltipTrigger asChild>
                        <div
                          className={`w-3 h-3 rounded-full ${blobColor} transition-all duration-300 hover:scale-150 cursor-pointer animate-fade-in`}
                          style={{ animationDelay: `${index * 5}ms` }}
                        />
                      </TooltipTrigger>
                      <TooltipContent>
                        <p className="font-semibold">{day.date}</p>
                        <p className="text-xs">{label}</p>
                      </TooltipContent>
                    </Tooltip>
                  );
                })}
              </div>
            </TooltipProvider>

            <div className="flex items-center justify-center gap-6 mt-4 pt-4 border-t border-white/10 text-xs z-10 relative">
              <div className="flex items-center gap-2">
                <div className="w-3 h-3 rounded-full bg-green-500" />
                <span className="text-white/70">Operational</span>
              </div>
              <div className="flex items-center gap-2">
                <div className="w-3 h-3 rounded-full bg-yellow-400" />
                <span className="text-white/70">Degraded</span>
              </div>
              <div className="flex items-center gap-2">
                <div className="w-3 h-3 rounded-full bg-red-500" />
                <span className="text-white/70">Outage</span>
              </div>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
};
