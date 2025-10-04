"use client";

import { useMemo } from "react";
import ClientOnly from "@/components/ClientOnly";
import { Button } from "@/components/ui/button";
import { motion } from "framer-motion";
import { Sparkles, Ghost, RefreshCcw } from "lucide-react";
import Link from "next/link";

export default function NotFound() {
  const sparkles = useMemo(() => {
    return [...Array(20)].map(() => ({
      top: `${Math.random() * 100}%`,
      left: `${Math.random() * 100}%`,
      delay: Math.random() * 2,
      duration: 5 + Math.random() * 3,
    }));
  }, []);

  return (
    <div className="min-h-screen bg-gradient-to-br from-black via-zinc-900 to-neutral-900 flex flex-col items-center justify-center text-white relative overflow-hidden">
      <ClientOnly>
        <motion.div
          className="absolute top-0 left-0 w-full h-full pointer-events-none"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 1 }}
        >
          {sparkles.map((s, i) => (
            <motion.div
              key={i}
              className="absolute text-white"
              style={{ top: s.top, left: s.left }}
              animate={{ y: [-20, 20, -20] }}
              transition={{
                duration: s.duration,
                repeat: Infinity,
                ease: "easeInOut",
                delay: s.delay,
              }}
            >
              <Sparkles className="text-pink-500/50 w-5 h-5 blur-sm" />
            </motion.div>
          ))}
        </motion.div>
      </ClientOnly>

      <motion.div
        initial={{ scale: 0.8, opacity: 0 }}
        animate={{ scale: 1, opacity: 1 }}
        transition={{ duration: 0.6, type: "spring" }}
        className="z-10 text-center px-6"
      >
        <Ghost className="mx-auto mb-4 w-20 h-20 animate-pulse text-zinc-200" />
        <h1 className="text-4xl font-bold sm:text-6xl mb-2">
          404: Page Not Found
        </h1>
        <p className="text-lg text-zinc-400 mb-8">
          Looks like you hit a ghost zone 👻
        </p>

        <div className="flex flex-col sm:flex-row gap-4 justify-center">
          <Link href="/">
            <Button variant="default" size="lg" className="gap-2">
              <RefreshCcw className="w-4 h-4" /> Back to Reality
            </Button>
          </Link>
          <Link href="/contact">
            <Button variant="outline" size="lg" className="gap-2">
              <Ghost className="w-4 h-4" /> Report a Glitch
            </Button>
          </Link>
        </div>
      </motion.div>
    </div>
  );
}
