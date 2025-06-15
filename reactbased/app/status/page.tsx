"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
} from "@/components/ui/card";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from "recharts";

type MetricPoint = {
  time: string; // e.g. "12:00", "13:00"
  value: number;
};

type Node = {
  id: string;
  name: string;
  status: "online" | "offline" | "degraded";
  ip: string;
  location: string;
  uptime: string;
  load: number;
  loadHistory: MetricPoint[];
  uptimeHistory: MetricPoint[];
};

type Cluster = {
  id: string;
  name: string;
  status: "healthy" | "warning" | "critical";
  nodes: Node[];
};

const exampleClusters: Cluster[] = [
  {
    id: "cluster-1",
    name: "US-East Cluster",
    status: "healthy",
    nodes: [
      {
        id: "node-1",
        name: "Node A",
        status: "online",
        ip: "192.168.1.1",
        location: "Virginia, USA",
        uptime: "12 days 4 hours",
        load: 35,
        loadHistory: [
          { time: "08:00", value: 20 },
          { time: "09:00", value: 25 },
          { time: "10:00", value: 30 },
          { time: "11:00", value: 40 },
          { time: "12:00", value: 35 },
        ],
        uptimeHistory: [
          { time: "08:00", value: 99.9 },
          { time: "09:00", value: 99.95 },
          { time: "10:00", value: 99.98 },
          { time: "11:00", value: 99.99 },
          { time: "12:00", value: 99.97 },
        ],
      },
      {
        id: "node-2",
        name: "Node B",
        status: "degraded",
        ip: "192.168.1.2",
        location: "Virginia, USA",
        uptime: "8 days 11 hours",
        load: 65,
        loadHistory: [
          { time: "08:00", value: 50 },
          { time: "09:00", value: 55 },
          { time: "10:00", value: 70 },
          { time: "11:00", value: 75 },
          { time: "12:00", value: 65 },
        ],
        uptimeHistory: [
          { time: "08:00", value: 98 },
          { time: "09:00", value: 97.5 },
          { time: "10:00", value: 97 },
          { time: "11:00", value: 96.5 },
          { time: "12:00", value: 96 },
        ],
      },
    ],
  },
  {
    id: "cluster-2",
    name: "Europe Cluster",
    status: "warning",
    nodes: [
      {
        id: "node-3",
        name: "Node C",
        status: "offline",
        ip: "10.0.0.1",
        location: "Frankfurt, Germany",
        uptime: "0",
        load: 0,
        loadHistory: [],
        uptimeHistory: [],
      },
      {
        id: "node-4",
        name: "Node D",
        status: "online",
        ip: "10.0.0.2",
        location: "London, UK",
        uptime: "20 days 2 hours",
        load: 42,
        loadHistory: [
          { time: "08:00", value: 35 },
          { time: "09:00", value: 40 },
          { time: "10:00", value: 45 },
          { time: "11:00", value: 43 },
          { time: "12:00", value: 42 },
        ],
        uptimeHistory: [
          { time: "08:00", value: 99.8 },
          { time: "09:00", value: 99.85 },
          { time: "10:00", value: 99.9 },
          { time: "11:00", value: 99.9 },
          { time: "12:00", value: 99.92 },
        ],
      },
      {
        id: "node-5",
        name: "Node E",
        status: "online",
        ip: "10.0.0.3",
        location: "Paris, France",
        uptime: "15 days 6 hours",
        load: 28,
        loadHistory: [
          { time: "08:00", value: 20 },
          { time: "09:00", value: 25 },
          { time: "10:00", value: 30 },
          { time: "11:00", value: 27 },
          { time: "12:00", value: 28 },
        ],
        uptimeHistory: [
          { time: "08:00", value: 99.95 },
          { time: "09:00", value: 99.96 },
          { time: "10:00", value: 99.97 },
          { time: "11:00", value: 99.95 },
          { time: "12:00", value: 99.94 },
        ],
      },
    ],
  },
];

function StatusDot({ status }: { status: string }) {
  let color = "bg-gray-400";
  if (status === "online" || status === "healthy") color = "bg-green-500";
  else if (status === "degraded" || status === "warning") color = "bg-yellow-500";
  else if (status === "offline" || status === "critical") color = "bg-red-500";

  return <span className={`inline-block w-3 h-3 rounded-full ${color}`} />;
}

const nodeVariants = {
  hidden: { opacity: 0, x: -20 },
  visible: { opacity: 1, x: 0, transition: { duration: 0.3 } },
  hover: { scale: 1.03, boxShadow: "0 8px 15px rgba(0,0,0,0.1)" },
};

function NodeCard({
  node,
  onClick,
}: {
  node: Node;
  onClick: (node: Node) => void;
}) {
  return (
    <motion.div
      variants={nodeVariants}
      initial="hidden"
      animate="visible"
      whileHover="hover"
      className="mb-4 cursor-pointer"
      onClick={() => onClick(node)}
    >
      <Card className="backdrop-blur-md bg-white/30 dark:bg-gray-900/30 border border-white/20">
        <CardContent className="flex flex-col gap-2">
          <div className="flex justify-between items-center">
            <div>
              <p className="font-semibold text-lg">{node.name}</p>
              <p className="text-sm text-muted-foreground">{node.ip}</p>
            </div>
            <StatusDot status={node.status} />
          </div>
          <div className="flex gap-4 h-24">
            <div className="flex-1">
              <p className="text-xs font-medium mb-1 text-gray-700 dark:text-gray-300">
                CPU Load %
              </p>
              {node.loadHistory.length ? (
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={node.loadHistory}>
                    <Line
                      type="monotone"
                      dataKey="value"
                      stroke="#22c55e" // green-500
                      strokeWidth={2}
                      dot={false}
                    />
                    <XAxis dataKey="time" hide />
                    <YAxis domain={[0, 100]} hide />
                    <Tooltip
                      contentStyle={{
                        backgroundColor: "rgba(255, 255, 255, 0.8)",
                        borderRadius: "4px",
                        border: "none",
                        color: "#000",
                      }}
                    />
                    <CartesianGrid strokeDasharray="3 3" />
                  </LineChart>
                </ResponsiveContainer>
              ) : (
                <p className="text-xs text-muted-foreground">No data</p>
              )}
            </div>

            <div className="flex-1">
              <p className="text-xs font-medium mb-1 text-gray-700 dark:text-gray-300">
                Uptime %
              </p>
              {node.uptimeHistory.length ? (
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={node.uptimeHistory}>
                    <Line
                      type="monotone"
                      dataKey="value"
                      stroke="#3b82f6" // blue-500
                      strokeWidth={2}
                      dot={false}
                    />
                    <XAxis dataKey="time" hide />
                    <YAxis domain={[90, 100]} hide />
                    <Tooltip
                      contentStyle={{
                        backgroundColor: "rgba(255, 255, 255, 0.8)",
                        borderRadius: "4px",
                        border: "none",
                        color: "#000",
                      }}
                    />
                    <CartesianGrid strokeDasharray="3 3" />
                  </LineChart>
                </ResponsiveContainer>
              ) : (
                <p className="text-xs text-muted-foreground">No data</p>
              )}
            </div>
          </div>
        </CardContent>
      </Card>
    </motion.div>
  );
}

const clusterVariants = {
  hidden: { opacity: 0, y: 20 },
  visible: { opacity: 1, y: 0, transition: { type: "spring", stiffness: 80, damping: 15 } },
};

const clusterContentVariants = {
  collapsed: { height: 0, opacity: 0, transition: { duration: 0.3 } },
  expanded: {
    height: "auto",
    opacity: 1,
    transition: { duration: 0.5, delayChildren: 0.2, staggerChildren: 0.1 },
  },
};

const modalBackdrop = {
  visible: { opacity: 1, backdropFilter: "blur(10px)" },
  hidden: { opacity: 0, backdropFilter: "blur(0px)" },
};

const modalContent = {
  hidden: { y: 50, opacity: 0 },
  visible: { y: 0, opacity: 1, transition: { type: "spring", stiffness: 120, damping: 20 } },
  exit: { y: 50, opacity: 0, transition: { duration: 0.2 } },
};

export default function EdgeNetworkStatus() {
  const [openClusters, setOpenClusters] = useState<string[]>([]);
  const [selectedNode, setSelectedNode] = useState<Node | null>(null);

  const toggleCluster = (id: string) => {
    setOpenClusters((prev) =>
      prev.includes(id) ? prev.filter((c) => c !== id) : [...prev, id]
    );
  };

  const closeModal = () => setSelectedNode(null);

  return (
    <main className="p-6 max-w-4xl mx-auto">
      <h1 className="mb-8 text-3xl font-bold">Edge Network Status</h1>
      {exampleClusters.map((cluster) => {
        const isOpen = openClusters.includes(cluster.id);
        return (
          <motion.div
            key={cluster.id}
            variants={clusterVariants}
            initial="hidden"
            animate="visible"
            className="mb-6"
          >
            <Card className="backdrop-blur-md bg-white/30 dark:bg-gray-900/30 border border-white/20">
              <CardHeader
                className="flex justify-between items-center cursor-pointer select-none"
                onClick={() => toggleCluster(cluster.id)}
              >
                <CardTitle>{cluster.name}</CardTitle>
                <StatusDot status={cluster.status} />
              </CardHeader>

              <AnimatePresence initial={false}>
                {isOpen && (
                  <motion.div
                    variants={clusterContentVariants}
                    initial="collapsed"
                    animate="expanded"
                    exit="collapsed"
                    className="overflow-hidden"
                  >
                    <CardContent className="flex flex-col gap-4">
                      {cluster.nodes.map((node) => (
                        <NodeCard
                          key={node.id}
                          node={node}
                          onClick={setSelectedNode}
                        />
                      ))}
                    </CardContent>
                  </motion.div>
                )}
              </AnimatePresence>
            </Card>
          </motion.div>
        );
      })}

      <AnimatePresence>
        {selectedNode && (
          <motion.div
            className="fixed inset-0 flex justify-center items-center z-50 bg-black/40 backdrop-blur-sm"
            variants={modalBackdrop}
            initial="hidden"
            animate="visible"
            exit="hidden"
            onClick={closeModal}
          >
            <motion.div
              className="bg-white/30 dark:bg-gray-900/30 rounded-lg shadow-xl max-w-2xl w-full p-6 relative border border-white/20 backdrop-blur-md"
              variants={modalContent}
              initial="hidden"
              animate="visible"
              exit="exit"
              onClick={(e) => e.stopPropagation()}
            >
              <button
                onClick={closeModal}
                className="absolute top-3 right-3 text-gray-500 hover:text-gray-900 dark:hover:text-white"
                aria-label="Close modal"
              >
                ✕
              </button>
              <h2 className="text-3xl font-semibold mb-6">{selectedNode.name}</h2>
              <p className="mb-2"><strong>Status:</strong> {selectedNode.status}</p>
              <p className="mb-2"><strong>IP Address:</strong> {selectedNode.ip}</p>
              <p className="mb-6"><strong>Location:</strong> {selectedNode.location}</p>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div>
                  <p className="font-semibold mb-2 text-gray-700 dark:text-gray-300">CPU Load History (%)</p>
                  {selectedNode.loadHistory.length ? (
                    <ResponsiveContainer width="100%" height={150}>
                      <LineChart data={selectedNode.loadHistory}>
                        <Line
                          type="monotone"
                          dataKey="value"
                          stroke="#22c55e"
                          strokeWidth={3}
                          dot={false}
                        />
                        <XAxis dataKey="time" />
                        <YAxis domain={[0, 100]} />
                        <Tooltip
                          contentStyle={{
                            backgroundColor: "rgba(255, 255, 255, 0.9)",
                            borderRadius: "4px",
                            border: "none",
                            color: "#000",
                          }}
                        />
                        <CartesianGrid strokeDasharray="3 3" />
                      </LineChart>
                    </ResponsiveContainer>
                  ) : (
                    <p className="text-sm text-muted-foreground">No load history available</p>
                  )}
                </div>

                <div>
                  <p className="font-semibold mb-2 text-gray-700 dark:text-gray-300">Uptime History (%)</p>
                  {selectedNode.uptimeHistory.length ? (
                    <ResponsiveContainer width="100%" height={150}>
                      <LineChart data={selectedNode.uptimeHistory}>
                        <Line
                          type="monotone"
                          dataKey="value"
                          stroke="#3b82f6"
                          strokeWidth={3}
                          dot={false}
                        />
                        <XAxis dataKey="time" />
                        <YAxis domain={[90, 100]} />
                        <Tooltip
                          contentStyle={{
                            backgroundColor: "rgba(255, 255, 255, 0.9)",
                            borderRadius: "4px",
                            border: "none",
                            color: "#000",
                          }}
                        />
                        <CartesianGrid strokeDasharray="3 3" />
                      </LineChart>
                    </ResponsiveContainer>
                  ) : (
                    <p className="text-sm text-muted-foreground">No uptime history available</p>
                  )}
                </div>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </main>
  );
}
