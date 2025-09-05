"use client";

import { AppSidebar } from "@/components/domain-sidebar";
import { DataTable } from "@/components/dns-table";
import { CreateRecordSheet } from "@/components/DNS-Create-Record-Sheet";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

export default function DNSPageContent({
  slug,
  data,
}: {
  slug: string;
  data: any[];
}) {
  const totalRecords = data.length;
  const rootRecords = data.filter((r) => r.subdomain === "@").length;
  const wwwRecords = data.filter((r) => r.subdomain === "www").length;

  return (
        <div className="flex flex-1 flex-col gap-4 py-4 md:gap-6 md:py-6 px-4 lg:px-6">
          
          {/* Info / Zone overview */}
          <Card>
            <CardHeader>
              <CardTitle>DNS</CardTitle>
              <CardDescription>
                Configure DNS records and review proxy status for your hostnames.
              </CardDescription>
            </CardHeader>
          </Card>

          {/* Recommended steps */}
          <Card>
            <CardHeader>
              <CardTitle>Recommended steps to complete zone setup</CardTitle>
            </CardHeader>
            <CardContent>
              <ul className="list-disc list-inside space-y-1">
                <li>Add an A, AAAA, or CNAME record for <strong>www</strong> so that <strong>www.{slug}</strong> resolves.</li>
                <li>Add an A, AAAA, or CNAME record for your root domain so that <strong>{slug}</strong> resolves.</li>
                <li>Add an MX record for your root domain so that mail can reach <strong>@{slug}</strong> addresses or set up SPF, DKIM, and DMARC records.</li>
              </ul>
            </CardContent>
          </Card>

          {/* Quick stats */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <Card>
              <CardContent>
                <p className="text-sm text-gray-500">Total Records</p>
                <p className="text-2xl font-bold">{totalRecords}</p>
              </CardContent>
            </Card>
            <Card>
              <CardContent>
                <p className="text-sm text-gray-500">Root (@) Records</p>
                <p className="text-2xl font-bold">{rootRecords}</p>
              </CardContent>
            </Card>
            <Card>
              <CardContent>
                <p className="text-sm text-gray-500">WWW Records</p>
                <p className="text-2xl font-bold">{wwwRecords}</p>
              </CardContent>
            </Card>
          </div>

          {/* Table */}
          <Card>
            <CardContent>
              <div className="flex justify-between items-center mb-4">
                <h2 className="text-2xl font-semibold tracking-tight text-foreground">
                  DNS Records
                </h2>
                <CreateRecordSheet />
              </div>
              <DataTable data={data} />
            </CardContent>
          </Card>
        </div>

  );
}
