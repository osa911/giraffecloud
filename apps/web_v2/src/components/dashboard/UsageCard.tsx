"use client";

import useSWR from "swr";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { AlertTriangle, Loader2 } from "lucide-react";
import { fetcher } from "@/lib/swr-fetcher";
import { UsageData } from "@/types/tunnel";

interface UsageCardProps {
  monthlyLimitBytes?: number;
}

export default function UsageCard({ monthlyLimitBytes }: UsageCardProps) {
  const { data: usage, error, isLoading } = useSWR<UsageData>(
    "/usage/summary",
    fetcher
  );

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
  };

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Current Usage</CardTitle>
        </CardHeader>
        <CardContent className="flex justify-center py-6">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Current Usage</CardTitle>
        </CardHeader>
        <CardContent>
          <Alert variant="destructive">
            <AlertTriangle className="h-4 w-4" />
            <AlertTitle>Error</AlertTitle>
            <AlertDescription>Could not load usage data</AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    );
  }

  const usedBytes = usage?.month?.used_bytes || 0;
  // Default limit 10GB if not provided
  const limitBytes = monthlyLimitBytes || 10 * 1024 * 1024 * 1024;
  const percentage = Math.min(Math.round((usedBytes / limitBytes) * 100), 100);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Current Usage</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <div className="flex justify-between text-sm">
            <span className="text-muted-foreground">Used</span>
            <span className="font-medium">{formatBytes(usedBytes)}</span>
          </div>
          <Progress value={percentage} />
          <div className="flex justify-between text-xs text-muted-foreground">
            <span>0 B</span>
            <span>{formatBytes(limitBytes)} Limit</span>
          </div>
        </div>
        <p className="text-xs text-muted-foreground pt-2">
          Usage resets on the 1st of every month.
        </p>
      </CardContent>
    </Card>
  );
}
