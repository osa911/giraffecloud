"use client";

import useSWR from "swr";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { AlertTriangle, Loader2, Activity } from "lucide-react";
import { fetcher } from "@/lib/swr-fetcher";
import { DailyUsageHistory } from "@/types/tunnel";
import { format, subDays } from "date-fns";

export default function DailyUsageChart() {
  const { data: usage, error, isLoading } = useSWR<DailyUsageHistory>(
    "/usage/daily-history?days=30",
    fetcher
  );

  const data = usage?.history.map((entry) => ({
    date: format(new Date(entry.date), "MMM dd"),
    bytes_in: entry.bytes_in,
    bytes_out: entry.bytes_out,
  })) || [];

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
  };

  if (isLoading) {
    return (
      <Card className="col-span-4">
        <CardHeader>
          <CardTitle>Traffic History (Last 30 Days)</CardTitle>
        </CardHeader>
        <CardContent className="flex justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card className="col-span-4">
        <CardHeader>
          <CardTitle>Traffic History</CardTitle>
        </CardHeader>
        <CardContent>
          <Alert variant="destructive">
            <AlertTriangle className="h-4 w-4" />
            <AlertTitle>Error</AlertTitle>
            <AlertDescription>Could not load usage history</AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="col-span-4">
      <CardHeader>
        <CardTitle>Traffic History (Last 30 Days)</CardTitle>
      </CardHeader>
      <CardContent className="pl-2">
        <div className="h-[300px] w-full relative">
          {(!usage || usage.history.length === 0) && (
            <div className="absolute inset-0 flex flex-col items-center justify-center bg-background/50 backdrop-blur-[1px] z-10 rounded-md border border-dashed">
              <div className="flex flex-col items-center space-y-2 text-muted-foreground">
                <Activity className="h-8 w-8 opacity-50" />
                <p className="font-medium">No traffic recorded yet</p>
                <p className="text-xs">Connect a tunnel to start seeing data</p>
              </div>
            </div>
          )}
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data} margin={{ top: 10, right: 30, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="colorBytesIn" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="var(--primary)" stopOpacity={0.8} />
                  <stop offset="95%" stopColor="var(--primary)" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="colorBytesOut" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#10b981" stopOpacity={0.8} />
                  <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis
                dataKey="date"
                stroke="var(--muted-foreground)"
                fontSize={12}
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                stroke="var(--muted-foreground)"
                fontSize={12}
                tickLine={false}
                axisLine={false}
                tickFormatter={(value) => formatBytes(value)}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: "var(--popover)",
                  borderColor: "var(--border)",
                  color: "var(--popover-foreground)",
                  borderRadius: "var(--radius)",
                }}
                itemStyle={{ color: "var(--foreground)" }}
                formatter={(value: number, name: string) => [
                  formatBytes(value),
                  name === "bytes_in" ? "Incoming" : "Outgoing",
                ]}
                labelStyle={{ color: "var(--muted-foreground)" }}
              />
              <CartesianGrid strokeDasharray="3 3" className="stroke-muted/20" vertical={false} />
              <Area
                type="monotone"
                dataKey="bytes_in"
                name="bytes_in"
                stroke="var(--primary)"
                fillOpacity={1}
                fill="url(#colorBytesIn)"
                stackId="1"
              />
              <Area
                type="monotone"
                dataKey="bytes_out"
                name="bytes_out"
                stroke="#10b981"
                fillOpacity={1}
                fill="url(#colorBytesOut)"
                stackId="2"
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </CardContent>
    </Card>
  );
}
