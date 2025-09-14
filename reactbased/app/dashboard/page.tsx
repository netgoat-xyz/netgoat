"use client";
import { AppSidebar } from "@/components/home-sidebar";
import { DataTable } from "@/components/domains-table";
import SiteHeader from "@/components/site-header";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import axios from "axios";

import React, { useEffect, useState } from "react";

type Domain = {
  group: string;
  name: string;
  status: "active" | "inactive" | "pending";
  lastSeen: string;
};

type Role = "user" | "admin";

type Data = {
  username: string;
  email: string;
  role: Role[]; // array of roles
  domains: Domain[];
  _id: string;
  createdAt: string; // or Date if you're parsing it
  updatedAt: string; // or Date if you're parsing it
};
export default function Page() {
  const [data, setData] = useState<Data | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const sessionStr = localStorage.getItem("session");
    if (!sessionStr) {
      setLoaded(true);
      setError("No session found. Please log in again.");
      return;
    }
    let lsd: any = {};
    try {
      lsd = JSON.parse(sessionStr || "{}");
    } catch (e) {
      setLoaded(true);
      setError("Session data corrupted. Please log in again.");
      return;
    }
    const id = lsd.userId;
    if (!id) {
      setLoaded(true);
      setError("Session missing user ID. Please log in again.");
      return;
    }
    axios
      .get(`${process.env.backendapi}/api/${id}`, {
        withCredentials: true,
        headers: {
          Authorization: `Bearer ${localStorage.getItem("jwt")}`,
        },
      })
      .then((res) => {
        setData(res.data || null);
        localStorage.setItem("userFD", res.data);
        setLoaded(true);
        setError(null);
      })
      .catch((err) => {
        console.error("Domain fetch failed", err);
        setLoaded(true);
        setError("Failed to fetch domains. Please try again.");
      });
  }, []);

  // Interactive loading/error overlay (hidden after loaded)
  const LoadingOverlay = (
    <div
      className={`fixed inset-0 z-50 flex flex-col items-center justify-center bg-background/80 backdrop-blur-sm animate-fade-in pointer-events-auto transition-opacity duration-500 ${
        loaded ? "opacity-0 pointer-events-none" : "opacity-100"
      }`}
      style={{ visibility: loaded ? "hidden" : "visible" }}
    >
      <svg
        className="animate-spin mb-6"
        width="48"
        height="48"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <circle cx="12" cy="12" r="10" strokeOpacity="0.2" />
        <path d="M12 2a10 10 0 0 1 10 10" />
      </svg>
      <button
        className="px-4 py-2 rounded-lg border bg-muted/30 hover:bg-muted/50 font-medium text-base transition-colors"
        onClick={() => window.location.reload()}
      >
        {error ? "Retry" : "Still loading? Click to retry"}
      </button>
      <div className="mt-4 text-muted-foreground text-sm">
        {error ? error : "Fetching your domains..."}
      </div>
    </div>
  );

  // Debug: log data and error
  if (process.env.NODE_ENV !== "production") {
    console.log("Data fetched:", data);
    if (error) console.warn("Dashboard error:", error);
  }

  return (
    <>
<<<<<<< HEAD
      <div className="flex flex-1 flex-col">
        <div className="@container/main flex flex-1 flex-col gap-2">
          <div className="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
            <div className="px-4 lg:px-6">
              {/* Show error message if loaded but no data */}
              {loaded &&
              !error &&
              (!data || !data.domains || data.domains.length === 0) ? (
                <div className="text-center text-muted-foreground py-8 text-lg">
                  No domains found. Add a domain to get started.
=======
          <div className="flex flex-1 flex-col">
            <div className="@container/main flex flex-1 flex-col gap-2">
              <div className="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
                <div className="px-4 lg:px-6">
                  {/* Show error message if loaded but no data */}
                  {loaded && !error && (!data || !data.domains || data.domains.length === 0) ? (
                    <div className="text-center text-muted-foreground py-8 text-lg">
                      No domains found. Add a domain to get started.
                    </div>
                  ) : null}
                  {/* Show table if data exists */}
                  {loaded && !error && data && data.domains && data.domains.length > 0 ? (
                    <DataTable data={data.domains} />
                  ) : null}
                  {/* Show error message if loaded and error */}
                  {loaded && error ? (
                    <div className="text-center text-destructive py-8 text-lg">
                      {error}
                    </div>
                  ) : null}
>>>>>>> 1e26c937094b9bb52e60e9b85f0514df46ed7c2d
                </div>
              ) : null}
              {/* Show table if data exists */}
              {loaded &&
              !error &&
              data &&
              data.domains &&
              data.domains.length > 0 ? (
                <DataTable data={data.domains} />
              ) : null}
              {/* Show error message if loaded and error */}
              {loaded && error ? (
                <div className="text-center text-destructive py-8 text-lg">
                  {error}
                </div>
              ) : null}
            </div>
          </div>
<<<<<<< HEAD
        </div>
      </div>
=======
>>>>>>> 1e26c937094b9bb52e60e9b85f0514df46ed7c2d
      {LoadingOverlay}
    </>
  );
}
