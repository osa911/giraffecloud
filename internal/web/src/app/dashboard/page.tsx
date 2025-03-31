"use client";

import { useEffect, useState } from "react";
import {
  Box,
  Grid,
  Paper,
  Typography,
  Card,
  CardContent,
  CardHeader,
} from "@mui/material";
import { api } from "@/services/api";

interface DashboardStats {
  totalTunnels: number;
  activeTunnels: number;
  totalTraffic: number;
}

export default function Dashboard() {
  const [stats, setStats] = useState<DashboardStats>({
    totalTunnels: 0,
    activeTunnels: 0,
    totalTraffic: 0,
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchStats = async () => {
      try {
        const response = await api.get("/dashboard/stats");
        setStats(response.data);
      } catch (error) {
        // Error is handled by the API interceptor
      } finally {
        setLoading(false);
      }
    };

    fetchStats();
  }, []);

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Dashboard
      </Typography>
      <Grid container spacing={3}>
        <Grid item xs={12} md={4}>
          <Card>
            <CardHeader title="Total Tunnels" />
            <CardContent>
              <Typography variant="h3">
                {loading ? "..." : stats.totalTunnels}
              </Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} md={4}>
          <Card>
            <CardHeader title="Active Tunnels" />
            <CardContent>
              <Typography variant="h3">
                {loading ? "..." : stats.activeTunnels}
              </Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} md={4}>
          <Card>
            <CardHeader title="Total Traffic" />
            <CardContent>
              <Typography variant="h3">
                {loading ? "..." : `${stats.totalTraffic} GB`}
              </Typography>
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    </Box>
  );
}
