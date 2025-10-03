"use client";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Card, CardContent } from "@/components/ui/card";
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

const schema = z.object({
  group: z.string(),
  name: z.string(),
  status: z.enum(["active", "inactive", "pending"]),
  lastSeen: z.string(), // or z.date() if you parse it
});

type Row = z.infer<typeof schema>;

export function DataTable({ data }: { data: Array<Row> }) {

console.log("Full data:", data);
data.forEach((row, i) => {
  console.log(`Row ${i}:`, row);
});
   if (!data || data.length === 0) {
    return (
      <Card className="rounded-2xl py-0 mt-2 border border-border bg-background shadow-sm">
        <CardContent className="p-6 text-center text-muted-foreground">
          No domains found.
        </CardContent>
      </Card>
    );
  }
  return (
    <Card className="rounded-2xl py-0 mt-2 border border-border bg-background shadow-sm">
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="pl-6">Name</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Group</TableHead>
              <TableHead className="text-right pr-6">Last Seen</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.map((row) => (
              <TableRow
                key={row.name}
                className="hover:bg-muted/50 transition-colors"
              >
                <TableCell className="pl-6 font-medium">
                  <Link href={`/dashboard/${row.name}`}>{row.name}</Link>
                </TableCell>
                <TableCell>
                  <span
                    className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold ${
                      row.status === "active"
                        ? "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300"
                        : row.status === "inactive"
                        ? "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300"
                        : "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300"
                    }`}
                  >
                    {row.status}
                  </span>
                </TableCell>
                <TableCell>{row.group}</TableCell>
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
                      <DropdownMenuItem>
                        <Link href={`/dashboard/${row.name}/proxies`}>
                          Proxies
                        </Link>
                      </DropdownMenuItem>
                      <DropdownMenuItem>
                        <Link href={`/dashboard/${row.name}/certs/`}>Certs</Link>
                      </DropdownMenuItem>
                      <DropdownMenuItem>
                        <Link href={`/dashboard/${row.name}/access`}>Access</Link>
                      </DropdownMenuItem>
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
