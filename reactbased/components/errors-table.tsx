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

const schema = z.object({
  name: z.string(),
  status: z.enum(["fixed", "patching", "on-going"]),
  site: z.string(),
  users: z.number(),
  lastSeen: z.string().datetime(),
});

type Row = z.infer<typeof schema>;

export default function DataTable({ data }: { data: Array<Row> }) {
  return (
    <Card className="rounded-2xl py-0 mt-2 border border-border bg-background shadow-sm">
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="pl-6">Name</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Site</TableHead>
              <TableHead>Users</TableHead>
              <TableHead className="text-right pr-6">Last Seen</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.map((row) => (
              <TableRow
                key={row.name}
                className="hover:bg-muted/50 transition-colors"
              >
                <TableCell className="pl-6 font-medium"><Link href={`/dashboard/${row.name}`}>{row.name}</Link></TableCell>
                <TableCell>
                  <span
                    className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold ${
                      row.status === "fixed"
                        ? "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300"
                        : row.status === "on-going"
                        ? "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300"
                        : row.status === "patching"
                        ? "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300"
                        : ""
                    }`}
                  >
                    {row.status.charAt(0).toUpperCase() + row.status.slice(1)}
                  </span>
                </TableCell>
                <TableCell>{row.site}</TableCell>
                <TableCell>{row.users}</TableCell>
                <TableCell className="text-right pr-6 text-sm">{new Date(row.lastSeen).toLocaleString()}</TableCell>
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
                      <DropdownMenuLabel className="font-semibold text-foreground">{row.name}</DropdownMenuLabel>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem className="hover:text-destructive dark:hover:text-destructive opacity-100">Delete</DropdownMenuItem>
                      <DropdownMenuItem><Link href={`/dashboard/${row.name}/proxies`}>Proxies</Link></DropdownMenuItem>
                      <DropdownMenuItem><Link href={`/dashboard/${row.name}/certs/`}>Certs</Link></DropdownMenuItem>
                      <DropdownMenuItem><Link href={`/dashboard/${row.name}/access`}>Access</Link></DropdownMenuItem>
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
