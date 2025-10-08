"use client";

import { Box, Grid, Typography, Card, CardContent, CardHeader } from "@mui/material";
import UsageCard from "@/components/dashboard/UsageCard";
import DailyUsageChart from "@/components/dashboard/DailyUsageChart";

interface DashboardStats {
  totalTunnels: number;
  activeTunnels: number;
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
  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Dashboard
      </Typography>
      <Grid container spacing={3}>
        <Grid size={{ xs: 12, md: 4 }}>
          <Card>
            <CardHeader title="Total Tunnels" />
            <CardContent>
              <Typography variant="h3">{initialStats.totalTunnels}</Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <Card>
            <CardHeader title="Active Tunnels" />
            <CardContent>
              <Typography variant="h3">{initialStats.activeTunnels}</Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <Card>
            <CardHeader title="Total Traffic" />
            <CardContent>
              <Typography variant="h3">{formatBytes(initialStats.totalTraffic)}</Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <UsageCard monthlyLimitBytes={undefined} />
        </Grid>
        <Grid size={{ xs: 12 }}>
          <DailyUsageChart />
        </Grid>
      </Grid>
    </Box>
  );
}
