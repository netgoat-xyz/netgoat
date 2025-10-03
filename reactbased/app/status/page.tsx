"use client";

import { useEffect, useState } from "react";
import { StatusHeader } from "@/components/status/StatusHeader";
import { ServiceCard } from "@/components/status/ServiceCard";
import { StatusModal } from "@/components/status/StatusModal";
import { HistoryTimeline } from "@/components/status/HistoryTimeline";
import { AnnouncementBanner } from "@/components/status/AnnouncementBanner";
import { Activity } from "lucide-react";
import { motion } from "framer-motion";
import ShaderBackground from "@/components/homescreen/shader-background";
import Header from "@/components/homescreen/header";

interface Service {
  id: string;
  name: string;
  status: "operational" | "degraded" | "outage" | "maintenance";
  uptime: string;
  responseTime: string;
  description: string;
}

const Index = () => {
  const [services, setServices] = useState<Service[]>([]);
  const [selectedService, setSelectedService] = useState<Service | null>(null);
  const [loading, setLoading] = useState(true);

  // fetch and filter health pings
  const fetchServices = async () => {
    setLoading(true);
    try {
      const res = await fetch("http://localhost:1933/api/history");
      const data = await res.json();

      const healthPings = data.history.filter(
        (entry: any) => entry.status === "health_ping"
      );

      // group by service and get last status
      const grouped: Record<string, Service> = {};
      healthPings.forEach((entry: any) => {
        const serviceName = entry.service || "Unknown";
        grouped[serviceName] = {
          id: serviceName,
          name: serviceName,
          status: "operational", // placeholder, can map your response codes
          uptime: "99.99%", // placeholder
          responseTime: entry.responseTime + "ms",
          description: entry.details?.endpoint || "",
        };
      });

      setServices(Object.values(grouped));
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchServices();
  }, []);

  const overallStatus = services.some(s => s.status === "outage")
    ? "outage"
    : services.some(s => s.status === "degraded")
    ? "degraded"
    : services.some(s => s.status === "maintenance")
    ? "maintenance"
    : "operational";

  return (
    <ShaderBackground>
            <Header />
      
    <div className="relative min-h-screen text-white">
      <div className="max-w-7xl mx-auto px-4 py-10">
        {/* Header */}
        <motion.div
          className="flex items-center mb-10"
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6 }}
        >
          <div>
            <h1 className="text-3xl md:text-4xl font-extrabold drop-shadow-sm">
              Netgoat Status
            </h1>
            <p className="text-gray-300">
              Real-time monitoring of all services
            </p>
          </div>
        </motion.div>

        {/* Overall Status */}
        <StatusHeader status={overallStatus} />
        <AnnouncementBanner />

        {/* Services */}
        <div className="mb-16">
          <h2 className="text-2xl font-bold mb-6">Services</h2>
          {loading ? (
            <p>Loading services…</p>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
              {services.map((service, index) => (
                <motion.div
                  key={service.id}
                  initial={{ opacity: 0, scale: 0.9 }}
                  animate={{ opacity: 1, scale: 1 }}
                  transition={{
                    delay: index * 0.1,
                    type: "spring",
                    stiffness: 120,
                  }}
                  className=""
                >
                  <ServiceCard
                    name={service.name}
                    status={service.status}
                    uptime={service.uptime}
                    responseTime={service.responseTime}
                    onClick={() => setSelectedService(service)}
                    delay={index * 100}
                  />
                </motion.div>
              ))}
            </div>
          )}
        </div>

        {/* History Timeline */}
        <div className="p-6 rounded-3xl bg-white/[0.06] backdrop-blur-2xl border border-white/[0.08] hover:bg-white/[0.12] hover:border-white/[0.16] transition-all duration-500 ease-out h-full relative overflow-hidden transform flex flex-col">
          <h2 className="text-xl font-semibold mb-4">History</h2>
          <HistoryTimeline />
        </div>

        {/* Modal */}
        {selectedService && (
          <StatusModal
            isOpen={!!selectedService}
            onClose={() => setSelectedService(null)}
            service={selectedService}
          />
        )}
      </div>
    </div>
    </ShaderBackground>
  );
};

export default Index;
