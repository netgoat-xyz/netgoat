import { Card } from "@/components/ui/card";
import { Megaphone, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useState } from "react";

interface Announcement {
  id: string;
  title: string;
  message: string;
  date: string;
  type: "info" | "warning" | "success";
}

const announcements: Announcement[] = [
  {
    id: "1",
    title: "New API Version Released",
    message:
      "We've released v2.0 of our API with improved performance and new features. Check the documentation for migration guide.",
    date: "2 hours ago",
    type: "success",
  },
  {
    id: "2",
    title: "Upcoming Maintenance Window",
    message:
      "Scheduled maintenance on January 20th from 2:00 AM - 4:00 AM UTC. Services may be temporarily unavailable.",
    date: "1 day ago",
    type: "warning",
  },
];

export const AnnouncementBanner = () => {
  const [dismissed, setDismissed] = useState<string[]>([]);

  const visibleAnnouncements = announcements.filter(
    (a) => !dismissed.includes(a.id),
  );

  if (visibleAnnouncements.length === 0) return null;

  return (
    <div className="space-y-4 mb-8">
      <h2 className="text-2xl font-bold flex items-center gap-2">
        <Megaphone className="w-6 h-6 text-primary" />
        Announcements
      </h2>

      {visibleAnnouncements.map((announcement, index) => (
        <Card
          key={announcement.id}
          className={`p-6 rounded-3xl bg-white/[0.06] backdrop-blur-2xl border border-white/[0.08] hover:bg-white/[0.12] hover:border-white/[0.16] transition-all duration-500 ease-out h-full relative overflow-hidden transform hover:scale-[1.02] hover:-translate-y-1 flex flex-col`}
          style={{ animationDelay: `${index * 100}ms` }}
        >
          <div className="flex items-start justify-between gap-4">
            <div className="flex-1">
              <div className="flex items-center gap-2 mb-2">
                <h3 className="font-semibold text-lg">{announcement.title}</h3>
                <span className="text-xs text-muted-foreground">
                  {announcement.date}
                </span>
              </div>
              <p className="text-sm text-muted-foreground">
                {announcement.message}
              </p>
            </div>

            <Button
              variant="ghost"
              size="sm"
              className="h-8 w-8 p-0"
              onClick={() => setDismissed([...dismissed, announcement.id])}
            >
              <X className="w-4 h-4" />
            </Button>
          </div>
        </Card>
      ))}
    </div>
  );
};
