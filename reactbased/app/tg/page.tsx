"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useState } from "react";
import Link from "next/link";

const testingPages = [
  { name: "Theme Generator", path: "/testing-grounds/theme-generator" },
  { name: "New Component Sandbox", path: "/testing-grounds/component-sandbox" },
  { name: "Dynamic Form Test", path: "/testing-grounds/dynamic-form" },
];

export default function TestingGrounds() {
  const [search, setSearch] = useState("");

  const filtered = testingPages.filter((page) =>
    page.name.toLowerCase().includes(search.toLowerCase()),
  );

  return (
    <div className="p-8 space-y-6 max-w-3xl mx-auto">
      <Card>
        <CardHeader>
          <CardTitle>Testing Grounds</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <Input
            placeholder="Search testing pages..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <div className="grid gap-4">
            {filtered.map((page) => (
              <Card
                key={page.path}
                className="hover:shadow-lg transition-shadow"
              >
                <CardContent className="flex justify-between items-center">
                  <span>{page.name}</span>
                  <Link href={page.path}>
                    <Button variant="outline" size="sm">
                      Open
                    </Button>
                  </Link>
                </CardContent>
              </Card>
            ))}
            {filtered.length === 0 && (
              <p className="text-sm text-gray-500">No pages found</p>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
