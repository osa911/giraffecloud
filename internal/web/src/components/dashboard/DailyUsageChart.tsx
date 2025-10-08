"use client";

import { Card, CardHeader, CardContent, Box, Typography, useTheme } from "@mui/material";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from "recharts";
import useSWR from "swr";
import clientApi from "@/services/apiClient/clientApiClient";
import type { DailyUsageHistory } from "@/types/tunnel";

const fetcher = (endpoint: string) => {
  return clientApi().get<DailyUsageHistory>(endpoint);
};

function formatBytes(bytes: number): string {
  if (bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

function formatDate(dateStr: string): string {
  const date = new Date(dateStr);
  return date.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

interface CustomTooltipProps {
  active?: boolean;
  payload?: Array<{
    name: string;
    value: number;
    color: string;
  }>;
  label?: string;
}

function CustomTooltip({ active, payload, label }: CustomTooltipProps) {
  if (active && payload && payload.length) {
    return (
      <Box
        sx={{
          bgcolor: "background.paper",
          p: 1.5,
          border: 1,
          borderColor: "divider",
          borderRadius: 1,
        }}
      >
        <Typography variant="body2" fontWeight="bold" gutterBottom>
          {label}
        </Typography>
        {payload.map((entry, index) => (
          <Typography key={index} variant="body2" sx={{ color: entry.color }}>
            {entry.name}: {formatBytes(entry.value)}
          </Typography>
        ))}
        <Typography variant="body2" fontWeight="bold" sx={{ mt: 0.5 }}>
          Total: {formatBytes(payload.reduce((sum, entry) => sum + entry.value, 0))}
        </Typography>
      </Box>
    );
  }
  return null;
}

export default function DailyUsageChart() {
  const theme = useTheme();
  const { data, error, isLoading } = useSWR<DailyUsageHistory>(
    "/usage/daily-history?days=30",
    fetcher,
    {
      refreshInterval: 60000, // Refresh every minute
      revalidateOnFocus: false,
      dedupingInterval: 30000,
    },
  );

  if (error) {
    return (
      <Card>
        <CardHeader title="Usage History (Last 30 Days)" />
        <CardContent>
          <Typography color="error">Failed to load usage data</Typography>
        </CardContent>
      </Card>
    );
  }

  if (isLoading || !data) {
    return (
      <Card>
        <CardHeader title="Usage History (Last 30 Days)" />
        <CardContent>
          <Typography color="text.secondary">Loading...</Typography>
        </CardContent>
      </Card>
    );
  }

  // Prepare data for chart (convert bytes to MB for better readability)
  const chartData = data.history.map((entry) => ({
    date: formatDate(entry.date),
    "Incoming (MB)": Number((entry.bytes_in / (1024 * 1024)).toFixed(2)),
    "Outgoing (MB)": Number((entry.bytes_out / (1024 * 1024)).toFixed(2)),
    bytes_in: entry.bytes_in,
    bytes_out: entry.bytes_out,
  }));

  // Calculate total usage
  const totalBytes = data.history.reduce((sum, entry) => sum + entry.total, 0);

  return (
    <Card>
      <CardHeader
        title="Usage History (Last 30 Days)"
        subheader={`Total: ${formatBytes(totalBytes)}`}
      />
      <CardContent>
        <ResponsiveContainer width="100%" height={300}>
          <LineChart data={chartData}>
            <CartesianGrid strokeDasharray="3 3" stroke={theme.palette.divider} />
            <XAxis
              dataKey="date"
              stroke={theme.palette.text.secondary}
              style={{ fontSize: "0.75rem" }}
              interval="preserveStartEnd"
              minTickGap={30}
            />
            <YAxis
              stroke={theme.palette.text.secondary}
              style={{ fontSize: "0.75rem" }}
              label={{
                value: "MB",
                angle: -90,
                position: "insideLeft",
                style: { fill: theme.palette.text.secondary, fontSize: "0.75rem" },
              }}
            />
            <Tooltip content={<CustomTooltip />} />
            <Legend />
            <Line
              type="monotone"
              dataKey="Incoming (MB)"
              stroke={theme.palette.primary.main}
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 4 }}
            />
            <Line
              type="monotone"
              dataKey="Outgoing (MB)"
              stroke={theme.palette.secondary.main}
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 4 }}
            />
          </LineChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
