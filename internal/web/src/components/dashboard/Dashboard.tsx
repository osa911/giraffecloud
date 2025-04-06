"use client";

import {
  Box,
  Grid,
  Typography,
  Card,
  CardContent,
  CardHeader,
} from "@mui/material";

interface DashboardStats {
  totalTunnels: number;
  activeTunnels: number;
  totalTraffic: number;
}

interface DashboardClientProps {
  initialStats: DashboardStats;
}

export default function DashboardClient({
  initialStats,
}: DashboardClientProps) {
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
              <Typography variant="h3">
                {initialStats.totalTraffic} GB
              </Typography>
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    </Box>
  );
}
