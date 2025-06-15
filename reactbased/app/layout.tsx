"use client";
import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";
import KonamiEasterEgg from "@/components/egg/konami";
import { useState } from "react";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

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
        className={cyberpunk ? `cyberpunk ${geistSans.variable} ${geistMono.variable} duration-300 transition-all antialiased` : `${geistSans.variable} ${geistMono.variable} dark transition-all duration-300 antialiased`}
      >
        {children}
      </body>
    </html>
  );
}
