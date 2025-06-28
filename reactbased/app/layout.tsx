"use client";
import type { Metadata } from "next";
import "./globals.css";
import KonamiEasterEgg from "@/components/egg/konami";
import { useState } from "react";

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const [cyberpunk, setCyberpunk] = useState(false)

  return (
    <html lang="en">
      <KonamiEasterEgg onActivate={() => setCyberpunk(!cyberpunk)} />

      <body
        className={cyberpunk ? `cyberpunk duration-300 transition-all antialiased` : `dark transition-all duration-300 antialiased`}
      >
        {children}
      </body>
    </html>
  );
}
