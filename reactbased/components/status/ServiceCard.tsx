import {
  CheckCircle2,
  AlertCircle,
  XCircle,
  Wrench,
  ChevronRight,
} from "lucide-react";
import { Card } from "@/components/ui/card";

interface ServiceCardProps {
  name: string;
  status: "operational" | "degraded" | "outage" | "maintenance";
  uptime: string;
  responseTime: string;
  onClick: () => void;
  delay?: number;
}

const statusConfig = {
  operational: {
    icon: CheckCircle2,
    label: "All Systems Operational",
    description: "All services are running smoothly",
    color: "text-green-400",
    bgColor: "bg-green-900/30",
    borderColor: "border-green-700",
  },
  degraded: {
    icon: AlertCircle,
    label: "Partial System Outage",
    description: "Some services may be experiencing issues",
    color: "text-yellow-400",
    bgColor: "bg-yellow-900/30",
    borderColor: "border-yellow-700",
  },
  outage: {
    icon: XCircle,
    label: "Major System Outage",
    description: "We're experiencing significant issues",
    color: "text-red-400",
    bgColor: "bg-red-900/30",
    borderColor: "border-red-700",
  },
  maintenance: {
    icon: Wrench,
    label: "Scheduled Maintenance",
    description: "Performing scheduled system updates",
    color: "text-blue-400",
    bgColor: "bg-blue-900/30",
    borderColor: "border-blue-700",
  },
};
export const ServiceCard = ({
  name,
  status,
  uptime,
  responseTime,
  onClick,
  delay = 0,
}: ServiceCardProps) => {
  const config = statusConfig[status];
  const Icon = config.icon;

  return (
    <Card
      className={`p-6 rounded-3xl bg-white/[0.06] backdrop-blur-2xl border border-white/[0.08] hover:bg-white/[0.12] hover:border-white/[0.16] transition-all duration-500 ease-out h-full relative overflow-hidden transform hover:scale-[1.02] hover:-translate-y-1 flex flex-col`}
      style={{ animationDelay: `${delay}ms` }}
      onClick={onClick}
    >
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center gap-3">
          <div
            className={`${config.bgColor} ${config.borderColor} border-2 p-2 rounded-lg`}
          >
            <Icon className={`w-5 h-5 ${config.color}`} />
          </div>
          <div>
            <h3 className="font-semibold text-lg -mb-2">{name}</h3>
            <span className={`text-xs ${config.color} font-medium`}>
              {config.label}
            </span>
          </div>
        </div>
        <ChevronRight className="w-5 h-5 text-muted-foreground transition-transform group-hover:translate-x-1" />
      </div>

      <div className="grid grid-cols-2 gap-4 mt-4 pt-4 border-white/20 border-t">
        <div>
          <p className="text-xs text-muted-foreground mb-1">Uptime</p>
          <p className="font-semibold text-foreground">{uptime}</p>
        </div>
        <div>
          <p className="text-xs text-muted-foreground mb-1">Response Time</p>
          <p className="font-semibold text-foreground">{responseTime}</p>
        </div>
      </div>
    </Card>
  );
};
