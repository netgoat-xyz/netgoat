"use client";

import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { z } from "zod";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import Link from "next/link";
import router from "next/dist/client/router";
import { CloudIcon } from "lucide-react";

const schema = z.object({
  type: z.string(),
  name: z.string(),
  content: z.string(),
  status: z.enum(["proxied", "unproxied"]),
  ttl: z.string(),
});

type Row = z.infer<typeof schema>;

export function DataTable({ data }: { data: readonly Row[] }) {
  return (
    <Card className="rounded-2xl py-0 mt-2 border border-border bg-background shadow-sm">
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="pl-6">Type</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Content</TableHead>
              <TableHead>Proxy Status</TableHead>
              <TableHead>TTL</TableHead>
              <TableHead className="text-right pr-6">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.map((row) => (
              <TableRow
                key={row.name}
                //
                // onClick={() => router.push(`/dashboard/${row.name}`)}
                className="hover:bg-muted/50 transition-colors"
              >
                <TableCell className="pl-6 font-medium">{row.type}</TableCell>
                <TableCell>{row.name}</TableCell>
                <TableCell>{row.content}</TableCell>
                <TableCell>
                  <span
                    className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-semibold ${
                      row.status === "proxied"
                        ? "bg-orange-100 text-green-800 dark:bg-green-900/40 dark:text-green-300"
                        : row.status === "unproxied"
                        ? "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300"
                        : "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300"
                    }`}
                  >
                    {row.status === "proxied" && (
                      <CloudIcon className="h-4 w-4 text-green-500" />
                    )}
                    {row.status}
                  </span>
                </TableCell>
                <TableCell>{row.ttl}</TableCell>
                <TableCell className="text-right pr-6 text-sm flex justify-end text-muted-foreground">
                  <DropdownMenu>
                    <DropdownMenuTrigger className="text-center ">
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        fill="none"
                        viewBox="0 0 24 24"
                        strokeWidth={1.5}
                        stroke="currentColor"
                        className="size-6"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          d="M6.75 12a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0ZM12.75 12a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0ZM18.75 12a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Z"
                        />
                      </svg>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent className="opacity-60 filter backdrop-blur-md">
                      <DropdownMenuLabel className="font-semibold text-foreground">
                        {row.name}
                      </DropdownMenuLabel>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem className="hover:text-destructive dark:hover:text-destructive opacity-100">
                        Delete
                      </DropdownMenuItem>
                      <DropdownMenuItem>Rename</DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
