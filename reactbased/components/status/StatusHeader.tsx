import { CheckCircle2, AlertCircle, XCircle, Wrench } from "lucide-react";
import { Badge } from "@/components/ui/badge";

interface StatusHeaderProps {
  status: "operational" | "degraded" | "outage" | "maintenance";
}

const statusConfig = {
  operational: {
    icon: CheckCircle2,
    label: "All Systems Operational",
    description: "All services are running smoothly",
    iconColor: "text-green-400",
    bgColor: "bg-green-900/30",
    borderColor: "border-green-700",
    textColor: "text-green-400",
  },
  degraded: {
    icon: AlertCircle,
    label: "Partial System Outage",
    description: "Some services may be experiencing issues",
    iconColor: "text-yellow-300",
    bgColor: "bg-yellow-900/30",
    borderColor: "border-yellow-700",
    textColor: "text-yellow-300",
  },
  outage: {
    icon: XCircle,
    label: "Major System Outage",
    description: "We're experiencing significant issues",
    iconColor: "text-red-400",
    bgColor: "bg-red-900/30",
    borderColor: "border-red-700",
    textColor: "text-red-400",
  },
  maintenance: {
    icon: Wrench,
    label: "Scheduled Maintenance",
    description: "Performing scheduled system updates",
    iconColor: "text-blue-400",
    bgColor: "bg-blue-900/30",
    borderColor: "border-blue-700",
    textColor: "text-blue-400",
  },
};

export const StatusHeader = ({ status }: StatusHeaderProps) => {
  const config = statusConfig[status];
  const Icon = config.icon;

  return (
    <div className="animate-fade-in">
      <div className="p-6 rounded-3xl bg-white/5 backdrop-blur-2xl border border-white/10 hover:bg-white/10 hover:border-white/20 mb-8 transition-all duration-500 ease-out h-full relative overflow-hidden transform hover:scale-[1.02] hover:-translate-y-1 flex flex-col">
        <div className="flex items-center gap-4 mb-4">
          <div className={`${config.bgColor} p-3 rounded-xl animate-pulse-slow flex items-center justify-center`}>
            <Icon className={`w-8 h-8 ${config.iconColor}`} />
          </div>
          <div className="flex-1">
            <h1 className={`text-3xl font-bold text-white -mb-0.5`}>
              {config.label}
            </h1>
            <p className="text-white/65">{config.description}</p>
          </div>
          <Badge
            variant="outline"
            className={`${config.textColor} border ${config.borderColor} px-4 py-2 text-sm font-semibold`}
          >
            Live Status
          </Badge>
        </div>
      </div>
    </div>
  );
};
