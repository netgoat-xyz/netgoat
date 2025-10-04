"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { AnimatePresence, motion } from "framer-motion";
import useSwipeDirection from "./useSwipeDirection";
import { Toaster } from "@/components/ui/sonner";

export default function DashboardClientWrapper({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const direction = useSwipeDirection();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const jwt = localStorage.getItem("jwt");
    if (!jwt) {
      router.replace("/auth");
      return;
    }
    fetch("/api/session?session=" + jwt)
      .then((res) => res.json())
      .then((data) => {
        localStorage.setItem("session", JSON.stringify(data));
        console.log(data);
        if (data.role != "admin") {
          router.replace("/");
          return;
        }
        setReady(true);
      })
      .catch(() => router.replace("/auth"));
  }, [router]);

  if (!ready) {
    return (
      <div className="flex h-screen items-center justify-center text-muted-foreground">
        Loading admin...
      </div>
    );
  }

  const variants = {
    enter: (dir: string) => ({
      x: dir === "right" ? 100 : -100,
      opacity: 0,
    }),
    center: {
      x: 0,
      opacity: 1,
      transition: { duration: 0.25 },
    },
    exit: (dir: string) => ({
      x: dir === "right" ? -100 : 100,
      opacity: 0,
      transition: { duration: 0.25 },
    }),
  };

  return (
    <div className="min-h-screen bg-background">
      <AnimatePresence mode="wait" custom={direction}>
        <motion.div
          key={pathname}
          custom={direction}
          variants={variants}
          initial="enter"
          animate="center"
          exit="exit"
        >
          {children}
          <Toaster position="top-right" richColors />
        </motion.div>
      </AnimatePresence>
    </div>
  );
}
