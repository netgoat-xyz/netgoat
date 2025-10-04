import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Copy, ExternalLink, ArrowLeft } from "lucide-react";
import { IconTrendingDown, IconTrendingUp } from "@tabler/icons-react";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { PageTitle } from "../SiteTitle";

export function OverviewContent() {
  return (
    <div>
      <div className="p-6">
        <PageTitle
          title="System Overview"
          subtitle="Key metrics and insights at a glance"
        />
        <div className="grid grid-cols-4 space-x-4 my-4">
          <Card className="@container/card">
            <CardHeader>
              <CardDescription>Total Requests Handled</CardDescription>
              <CardTitle className="text-2xl font-semibold tabular-nums @[250px]/card:text-3xl">
                500000
              </CardTitle>
              <CardAction>
                <Badge className="bg-green-100 text-green-800 dark:bg-green-800/30 dark:text-green-500">
                  <IconTrendingUp />
                  +12.5%
                </Badge>
              </CardAction>
            </CardHeader>
            <CardFooter className="flex-col items-start gap-1.5 text-sm">
              <div className="line-clamp-1 flex gap-2 font-medium">
                Trending up this month <IconTrendingUp className="size-4" />
              </div>
              <div className="text-muted-foreground">
                Requests globally for all users
              </div>
            </CardFooter>
          </Card>
          <Card className="@container/card">
            <CardHeader>
              <CardDescription>New Customers</CardDescription>
              <CardTitle className="text-2xl font-semibold tabular-nums @[250px]/card:text-3xl">
                1,234
              </CardTitle>
              <CardAction>
                <Badge variant="outline">
                  <IconTrendingDown />
                  -20%
                </Badge>
              </CardAction>
            </CardHeader>
            <CardFooter className="flex-col items-start gap-1.5 text-sm">
              <div className="line-clamp-1 flex gap-2 font-medium">
                Down 20% this period <IconTrendingDown className="size-4" />
              </div>
              <div className="text-muted-foreground">
                Acquisition needs attention
              </div>
            </CardFooter>
          </Card>
          <Card className="@container/card">
            <CardHeader>
              <CardDescription>Active Accounts</CardDescription>
              <CardTitle className="text-2xl font-semibold tabular-nums @[250px]/card:text-3xl">
                45,678
              </CardTitle>
              <CardAction>
                <Badge variant="outline">
                  <IconTrendingUp />
                  +12.5%
                </Badge>
              </CardAction>
            </CardHeader>
            <CardFooter className="flex-col items-start gap-1.5 text-sm">
              <div className="line-clamp-1 flex gap-2 font-medium">
                Strong user retention <IconTrendingUp className="size-4" />
              </div>
              <div className="text-muted-foreground">
                Engagement exceed targets
              </div>
            </CardFooter>
          </Card>
          <Card className="@container/card">
            <CardHeader>
              <CardDescription>Growth Rate</CardDescription>
              <CardTitle className="text-2xl font-semibold tabular-nums @[250px]/card:text-3xl">
                4.5%
              </CardTitle>
              <CardAction>
                <Badge variant="outline">
                  <IconTrendingUp />
                  +4.5%
                </Badge>
              </CardAction>
            </CardHeader>
            <CardFooter className="flex-col items-start gap-1.5 text-sm">
              <div className="line-clamp-1 flex gap-2 font-medium">
                Steady performance increase{" "}
                <IconTrendingUp className="size-4" />
              </div>
              <div className="text-muted-foreground">
                Meets growth projections
              </div>
            </CardFooter>
          </Card>
        </div>
      </div>
    </div>
  );
}
