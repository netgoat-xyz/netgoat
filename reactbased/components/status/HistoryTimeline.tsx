import { Card } from "@/components/ui/card";
import { CheckCircle2, AlertCircle, Wrench } from "lucide-react";

interface HistoryItem {
  id: string;
  date: string;
  title: string;
  description: string;
  status: "resolved" | "ongoing" | "maintenance";
}

const historyItems: HistoryItem[] = [
  {
    id: "1",
    date: "Jan 15, 2025",
    title: "Database Migration Completed",
    description: "Successfully migrated to new database infrastructure with improved performance.",
    status: "resolved",
  },
  {
    id: "2",
    date: "Jan 10, 2025",
    title: "API Latency Issues",
    description: "Experienced increased API response times. Issue was identified and resolved.",
    status: "resolved",
  },
  {
    id: "3",
    date: "Jan 5, 2025",
    title: "Scheduled Maintenance",
    description: "Performed routine system updates and security patches.",
    status: "maintenance",
  },
  {
    id: "4",
    date: "Dec 28, 2024",
    title: "CDN Performance Degradation",
    description: "CDN provider experienced brief outage affecting content delivery.",
    status: "resolved",
  },
];
const statusConfig = {
  resolved: {
    icon: CheckCircle2,
    color: "text-green-400",
    bgColor: "bg-green-900/30",
    borderColor: "border-green-700/30",
  },
  ongoing: {
    icon: AlertCircle,
    color: "text-yellow-400",
    bgColor: "bg-yellow-900/30",
    borderColor: "border-yellow-700/30",
  },
  maintenance: {
    icon: Wrench,
    color: "text-blue-400",
    bgColor: "bg-blue-900/30",
    borderColor: "border-blue-700/30",
  },
};


export const HistoryTimeline = () => {
  return (
    <div className="space-y-4 ">      
      <div className="relative">
        
        <div className="space-y-6">
{historyItems.map((item, index) => {
  const config = statusConfig[item.status];
  const Icon = config.icon;

  return (
    <Card
      key={item.id}
      className="p-6 rounded-3xl bg-white/[0.06] backdrop-blur-2xl border border-white/[0.08] hover:bg-white/[0.12] hover:border-white/[0.16] transition-all duration-500 ease-out h-full relative overflow-hidden transform hover:scale-[1.02] hover:-translate-y-1 flex flex-col"
      style={{ animationDelay: `${index * 100}ms` }}
    >
      {/* Icon on timeline line */}
      <div className={`absolute left-3 top-6 ${config.bgColor} ${config.borderColor} border-2 p-2 rounded-lg`}>
        <Icon className={`w-5 h-5 ${config.color}`} />
      </div>

      {/* Content shifted right so it doesn't overlap timeline */}
      <div className="ml-14">
        <div className="flex items-center justify-between mb-2">
          <h3 className="font-semibold text-lg">{item.title}</h3>
          <span className="text-sm text-muted-foreground whitespace-nowrap">{item.date}</span>
        </div>
        <p className="text-sm text-muted-foreground">{item.description}</p>
      </div>
    </Card>
  );
})}
        </div>
      </div>
    </div>
  );
};
