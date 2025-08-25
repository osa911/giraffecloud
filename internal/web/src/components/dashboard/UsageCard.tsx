"use client";
import { Card, CardHeader, CardContent, LinearProgress, Box, Typography } from "@mui/material";
import useSWR from "swr";
import clientApi from "@/services/apiClient/clientApiClient";

type UsageSummary = {
  period_start: string;
  bytes_in: number;
  bytes_out: number;
  requests: number;
};

const fetcher = (endpoint: string) => {
  return clientApi().get<UsageSummary>(endpoint);
};

export default function UsageCard({ monthlyLimitBytes }: { monthlyLimitBytes?: number }) {
  const { data } = useSWR<UsageSummary>("/usage/summary", fetcher, {
    refreshInterval: 15000,
    revalidateOnFocus: false,
    dedupingInterval: 5000,
    errorRetryInterval: 30000,
    errorRetryCount: 3,
  });
  const used = (data?.bytes_in ?? 0) + (data?.bytes_out ?? 0);
  const hasLimit = typeof monthlyLimitBytes === "number" && monthlyLimitBytes > 0;
  const pct = hasLimit
    ? Math.min(100, Math.round((used / (monthlyLimitBytes as number)) * 100))
    : 0;

  return (
    <Card>
      <CardHeader title="Usage" subheader="Current period" />
      <CardContent>
        <Box sx={{ mb: 1 }}>
          <LinearProgress variant={hasLimit ? "determinate" : "indeterminate"} value={pct} />
        </Box>
        <Typography variant="body2" color="text.secondary">
          {formatBytes(used)}
          {hasLimit ? ` of ${formatBytes(monthlyLimitBytes as number)} used` : " used"}
        </Typography>
      </CardContent>
    </Card>
  );
}

function formatBytes(bytes: number) {
  if (bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}
