"use client"

import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { z } from "zod"

const schema = z.object({
  group: z.string(),
  name: z.string(),
  status: z.enum(["active", "inactive", "pending"]),
  lastSeen: z.string(),
})

type Row = z.infer<typeof schema>

export function DataTable({ data }: { data: Array<Row> }) {
  return (
    <Card className="rounded-2xl border border-border bg-background shadow-sm">
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
              <TableRow key={row.name} className="hover:bg-muted/50 transition-colors">
                <TableCell className="pl-6 font-medium">{row.name}</TableCell>
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
                <TableCell className="text-right pr-6 text-sm text-muted-foreground">{row.lastSeen}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}
