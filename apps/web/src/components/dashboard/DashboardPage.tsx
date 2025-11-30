"use client";

import { useRouter } from "next/navigation";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  ArrowLeftRight,
  Zap,
  Activity,
  Rocket,
  ArrowRight,
} from "lucide-react";
import UsageCard from "@/components/dashboard/UsageCard";
import DailyUsageChart from "@/components/dashboard/DailyUsageChart";
import { ROUTES } from "@/constants/routes";

interface DashboardStats {
  totalTunnels: number;
  enabledTunnels: number;
  totalTraffic: number;
}

interface DashboardPageProps {
  initialStats: DashboardStats;
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

export default function DashboardPage({ initialStats }: DashboardPageProps) {
  const router = useRouter();

  return (
    <div className="flex-1 space-y-4">
      <div className="flex items-center justify-between space-y-2">
        <h2 className="text-3xl font-bold tracking-tight">Dashboard</h2>
        <div className="flex items-center space-x-2">
          {/* Add date range picker or download button here if needed */}
        </div>
      </div>
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Tunnels</CardTitle>
            <ArrowLeftRight className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{initialStats.totalTunnels}</div>
            <p className="text-xs text-muted-foreground">
              Active connections
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Enabled</CardTitle>
            <Zap className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{initialStats.enabledTunnels}</div>
            <p className="text-xs text-muted-foreground">
              Currently running
            </p>
          </CardContent>
        </Card>
        {/* Removed Total Traffic card as it is redundant with UsageCard */}
        {/* Placeholder for another stat or action */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Traffic</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatBytes(initialStats.totalTraffic)}
            </div>
            <p className="text-xs text-muted-foreground">
              Data transfer this month
            </p>
          </CardContent>
        </Card>
        {/* Placeholder for another stat or action */}
        <Card className="bg-primary text-primary-foreground">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-primary-foreground">Quick Action</CardTitle>
            <Rocket className="h-4 w-4 text-primary-foreground" />
          </CardHeader>
          <CardContent>
             <Button
                variant="secondary"
                size="sm"
                className="w-full mt-1 h-auto whitespace-normal py-2"
                onClick={() => router.push(ROUTES.DASHBOARD.TUNNELS)}
             >
                Manage Tunnels <ArrowRight className="ml-2 h-3 w-3 shrink-0" />
             </Button>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <DailyUsageChart />
        {/* <div className="col-span-4 lg:col-span-3">
             <UsageCard />
        </div> */}
      </div>
    </div>
  );
}
