
"use client";

import React, { useState, useMemo } from "react";
import { CalculatorIcon, ChevronDownIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Calendar } from "@/components/ui/calendar"
import { Label } from "@/components/ui/label"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"

import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableHeader,
  TableRow,
  TableHead,
  TableCell,
  TableBody,
} from "@/components/ui/table";
import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip as ReTooltip,
} from "recharts";
import { ChartContainer } from "@/components/ui/chart";
import { CalendarDateRangeIcon } from "@heroicons/react/24/solid";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectGroup, SelectLabel, SelectItem } from "@/components/ui/select";

// Custom tooltip styled like Vercel
function DarkTooltip({ active, payload, label }: any) {
  if (!active || !payload || !payload.length) return null;
  return (
    <div className="rounded-lg p-3 text-sm shadow-lg bg-card border border-zinc-800 min-w-[160px]">
      <div className="font-semibold mb-1 text-white">{label}</div>
      {payload.map((entry: any, i: number) => (
        <div key={i} className="flex justify-between text-gray-100">
          <span className="truncate mr-4">{entry.name}</span>
          <span className="font-medium" style={{ color: entry.color }}>
            {entry.value}
          </span>
        </div>
      ))}
    </div>
  );
}

export default function AnalyticsReplica() {
  const [timeframe, setTimeframe] = useState("7d");
  const [open, setOpen] = React.useState(false)
  const [date, setDate] = React.useState<Date | undefined>(undefined)

  const lineData = useMemo(
    () => [
      { name: "Sep 5", Visitors: 0 },
      { name: "Sep 6", Visitors: 70 },
      { name: "Sep 7", Visitors: 40 },
      { name: "Sep 8", Visitors: 50 },
      { name: "Sep 9", Visitors: 65 },
      { name: "Sep 10", Visitors: 45 },
      { name: "Sep 11", Visitors: 43 },
      { name: "Sep 12", Visitors: 30 },
    ],
    [timeframe]
  );

  return (
    <div className="flex flex-1 flex-col space-y-6 p-6">
      {/* Header */}
      <div className="flex items-center justify-between flex-wrap gap-4">
        <div>
          <h1 className="text-2xl font-bold text-white">Web Analytics</h1>
        </div>
      <div className="flex flex-col gap-3">
        
        <div className="flex items-center">
          <Select>
      <SelectTrigger className="w-[180px] mr-2">
        <SelectValue placeholder="All Subdomains" />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          <SelectItem value="all">all</SelectItem>
          <SelectItem value="@">@</SelectItem>
          <SelectItem value="www">www</SelectItem>
          <SelectItem value="api">api</SelectItem>
          <SelectItem value="canary">canary</SelectItem>
          <SelectItem value="beta">beta</SelectItem>
        </SelectGroup>
      </SelectContent>
    </Select>
                <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            variant="outline"
            id="date"
            className="border-t-border border-b-border border-l-border border-r-0 rounded-r-none"
          >
            <CalendarDateRangeIcon />
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-auto overflow-hidden p-0" align="start">
          <Calendar
            mode="single"
            selected={date}
            captionLayout="dropdown"
            onSelect={(date) => {
              setDate(date)
              setOpen(false)
            }}
          />
        </PopoverContent>
      </Popover>
        <Select>
      <SelectTrigger className="w-[180px] rounded-l-none mr-2">
        <SelectValue placeholder="Last 7 Days" />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          <SelectItem value="24h">Last 24 Hours</SelectItem>
          <SelectItem value="7d">Last 7 Days</SelectItem>
          <SelectItem value="30d">Last 30 Days</SelectItem>
          <SelectItem value="3mon">Last 3 Months</SelectItem>
          <SelectItem value="12mon">Last 12 Months</SelectItem>
        </SelectGroup>
      </SelectContent>
    </Select>
        </div>
    </div>
      </div>

      {/* Line Chart */}
      <div className="bg-card text-card-foreground flex flex-col gap-6 rounded-xl border shadow-sm">
        <CardContent className="-px-2">
          {/* Metric Cards */}
          <div className="grid grid-cols-1 bg-border sm:grid-cols-3 mb-4">
            <div className="bg-card/65 border-white border-b">
              <CardContent className="pt-6 pb-6 rounded-l-lg">
                <div className="text-sm  text-gray-300 -mt-1.5">Visitors</div>
                <div className="flex items-center space-x-3 mt-1.5">
                  <div className="text-3xl font-bold text-white">356</div>
                  <div className="bg-green-800/30 font-bold text-green-500 px-3 py-2 rounded-lg text-xs">
                    +1.6K%
                  </div>
                </div>
              </CardContent>
            </div>
            <div className="bg-card border-b border-border">
              <CardContent className="pt-6 rounded-l-lg">
                <div className="text-sm text-gray-300 -mt-1.5">Page Views</div>
                <div className="flex items-center space-x-3 mt-1.5">
                  <div className="text-3xl font-bold text-white">563</div>
                  <div className="bg-green-800/30 font-bold text-green-500 px-3 py-2 rounded-lg text-xs">
                    +1.8K%
                  </div>
                </div>{" "}
              </CardContent>
            </div>
            <div className="bg-card border-b border-border">
              <CardContent className="pt-6 rounded-l-lg">
                <div className="text-sm  text-gray-300 -mt-1.5">
                  Bounce Rate
                </div>
                <div className="flex items-center space-x-3 mt-1.5">
                  <div className="text-3xl font-bold text-white">71%</div>
                  <div className="bg-green-800/30 text-green-500 px-3 py-2 rounded-lg text-xs">
                    -2%
                  </div>
                </div>
              </CardContent>
            </div>
          </div>
        </CardContent>
        <ChartContainer
          config={{ Visitors: { color: "#3B82F6", label: "Visitors" } }} 
        >
          <ResponsiveContainer width="100%" height={280}>
            <LineChart data={lineData}>
              <XAxis
                dataKey="name"
                stroke={"#6b7280"} 
                tick={{ fontSize: 12, fill: "#374151" }}
                axisLine={false}
                tickLine={false}
              />
              <YAxis
                stroke={"#6b7280"}
                tick={{ fontSize: 12, fill: "#374151" }}
                axisLine={false}
                tickLine={false}
              />
              <CartesianGrid
                stroke={"#e5e7eb"}
                strokeDasharray="3 3"
                vertical={false}
              />
              <ReTooltip content={<DarkTooltip />} /> 
              subtle shadow, background matching theme
              <Line
                type="linear" 
                dataKey="Visitors"
                stroke="#3B82F6" 
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 4, fill: "#3B82F6", strokeWidth: 0 }}
              />
            </LineChart>
          </ResponsiveContainer>
        </ChartContainer>
      </div>

    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
      {/* Pages */}
      <Card>
        <CardHeader>
          <CardTitle className="text-white">Pages</CardTitle>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="routes">
            <TabsList className="mb-4">
              <TabsTrigger value="routes">Routes</TabsTrigger>
              <TabsTrigger value="hostnames">Hostnames</TabsTrigger>
            </TabsList>

            <TabsContent value="routes">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Path</TableHead>
                    <TableHead className="text-right">Visitors</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow>
                    <TableCell>/</TableCell>
                    <TableCell className="text-right">345</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>/register</TableCell>
                    <TableCell className="text-right">46</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>/dashboard</TableCell>
                    <TableCell className="text-right">41</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>/login</TableCell>
                    <TableCell className="text-right">7</TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </TabsContent>

            <TabsContent value="hostnames">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Hostname</TableHead>
                    <TableHead className="text-right">Visitors</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow>
                    <TableCell>app.duckbot.dev</TableCell>
                    <TableCell className="text-right">210</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>blog.duckbot.dev</TableCell>
                    <TableCell className="text-right">120</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>docs.duckbot.dev</TableCell>
                    <TableCell className="text-right">45</TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>

      {/* Referrers */}
      <Card>
        <CardHeader>
          <CardTitle className="text-white">Referrers</CardTitle>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="referrers">
            <TabsList className="mb-4">
              <TabsTrigger value="referrers">Referrers</TabsTrigger>
              <TabsTrigger value="utm">UTM Parameters</TabsTrigger>
            </TabsList>

            <TabsContent value="referrers">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Source</TableHead>
                    <TableHead className="text-right">Visitors</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow>
                    <TableCell>youtube.com</TableCell>
                    <TableCell className="text-right">104</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>google.com</TableCell>
                    <TableCell className="text-right">76</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>duckduckgo.com</TableCell>
                    <TableCell className="text-right">11</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>bing.com</TableCell>
                    <TableCell className="text-right">2</TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </TabsContent>

            <TabsContent value="utm">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>UTM</TableHead>
                    <TableHead className="text-right">Visitors</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow>
                    <TableCell>utm_source=twitter</TableCell>
                    <TableCell className="text-right">52</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>utm_campaign=launch</TableCell>
                    <TableCell className="text-right">38</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>utm_medium=email</TableCell>
                    <TableCell className="text-right">19</TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </div>


      {/* Countries / Devices / OS */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <Card>
          <CardHeader>
            <CardTitle className="text-white">Countries</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            <p className="text-gray-400">🇺🇸 United States — 20%</p>
            <p className="text-gray-400">🇩🇪 Germany — 12%</p>
            <p className="text-gray-400">🇫🇷 France — 5%</p>
            <p className="text-gray-400">🇷🇺 Russia — 4%</p>
            <p className="text-gray-400">🇮🇳 India — 4%</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-white">Devices</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            <p className="text-gray-400">Desktop — 59%</p>
            <p className="text-gray-400">Mobile — 39%</p>
            <p className="text-gray-400">Tablet — 2%</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-white">Operating Systems</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            <p className="text-gray-400">Windows — 31%</p>
            <p className="text-gray-400">Android — 22%</p>
            <p className="text-gray-400">Mac — 20%</p>
            <p className="text-gray-400">iOS — 19%</p>
            <p className="text-gray-400">Linux — 8%</p>
          </CardContent>
        </Card>
      </div>

      {/* Empty States */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card className="bg-card border-zinc-800 h-40 flex items-center justify-center">
          <p className="text-gray-500">No custom events</p>
        </Card>
        <Card className="bg-card border-zinc-800 h-40 flex items-center justify-center">
          <p className="text-gray-500">No flags</p>
        </Card>
      </div>
    </div>
  );
}
