"use client";

import { Box, Grid, Typography, Card, CardContent, Stack, Divider, Button } from "@mui/material";
import {
  VpnLock as TunnelIcon,
  Power as PowerIcon,
  DataUsage as DataUsageIcon,
  RocketLaunch as RocketLaunchIcon,
} from "@mui/icons-material";
import UsageCard from "@/components/dashboard/UsageCard";
import DailyUsageChart from "@/components/dashboard/DailyUsageChart";
import { useRouter } from "next/navigation";
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
    <Box>
      <Typography variant="h4" gutterBottom>
        Dashboard
      </Typography>
      <Grid container spacing={3}>
        {/* Combined Stats Card */}
        <Grid size={{ xs: 12, md: 8 }}>
          <Card>
            <CardContent>
              <Stack
                direction={{ xs: "column", sm: "row" }}
                spacing={{ xs: 3, sm: 4 }}
                divider={
                  <Divider
                    orientation="vertical"
                    flexItem
                    sx={{ display: { xs: "none", sm: "block" } }}
                  />
                }
              >
                <Box sx={{ flex: 1, textAlign: "center" }}>
                  <Stack
                    direction="row"
                    alignItems="center"
                    justifyContent="center"
                    spacing={1}
                    sx={{ mb: 1 }}
                  >
                    <TunnelIcon color="primary" />
                    <Typography variant="body2" color="text.secondary">
                      Total Tunnels
                    </Typography>
                  </Stack>
                  <Typography variant="h3">{initialStats.totalTunnels}</Typography>
                  <Divider sx={{ mt: 3, display: { xs: "block", sm: "none" } }} />
                </Box>
                <Box sx={{ flex: 1, textAlign: "center" }}>
                  <Stack
                    direction="row"
                    alignItems="center"
                    justifyContent="center"
                    spacing={1}
                    sx={{ mb: 1 }}
                  >
                    <PowerIcon color="success" />
                    <Typography variant="body2" color="text.secondary">
                      Enabled
                    </Typography>
                  </Stack>
                  <Typography variant="h3">{initialStats.enabledTunnels}</Typography>
                  <Divider sx={{ mt: 3, display: { xs: "block", sm: "none" } }} />
                </Box>
                <Box sx={{ flex: 1, textAlign: "center" }}>
                  <Stack
                    direction="row"
                    alignItems="center"
                    justifyContent="center"
                    spacing={1}
                    sx={{ mb: 1 }}
                  >
                    <DataUsageIcon color="info" />
                    <Typography variant="body2" color="text.secondary">
                      Total Traffic
                    </Typography>
                  </Stack>
                  <Typography variant="h3">{formatBytes(initialStats.totalTraffic)}</Typography>
                </Box>
              </Stack>
            </CardContent>
          </Card>
        </Grid>

        {/* Usage Card */}
        <Grid size={{ xs: 12, md: 4 }}>
          <UsageCard monthlyLimitBytes={undefined} />
        </Grid>

        {/* Quick Start Link */}
        {initialStats.totalTunnels === 0 && (
          <Grid size={{ xs: 12 }}>
            <Card
              sx={{
                background: "linear-gradient(135deg, #667eea 0%, #764ba2 100%)",
                color: "white",
              }}
            >
              <CardContent>
                <Stack
                  direction={{ xs: "column", sm: "row" }}
                  spacing={2}
                  alignItems={{ xs: "stretch", sm: "center" }}
                  justifyContent="space-between"
                >
                  <Stack spacing={1}>
                    <Stack direction="row" alignItems="center" spacing={1}>
                      <RocketLaunchIcon sx={{ fontSize: 32 }} />
                      <Typography variant="h5" fontWeight="bold">
                        Get Started with GiraffeCloud
                      </Typography>
                    </Stack>
                    <Typography variant="body1" sx={{ opacity: 0.95 }}>
                      New to GiraffeCloud? Follow our step-by-step guide to set up your first tunnel
                      in minutes.
                    </Typography>
                  </Stack>
                  <Button
                    variant="contained"
                    size="large"
                    onClick={() => router.push(ROUTES.DASHBOARD.GETTING_STARTED)}
                    sx={{
                      bgcolor: "white",
                      color: "primary.main",
                      "&:hover": { bgcolor: "grey.100" },
                      minWidth: { xs: "100%", sm: "auto" },
                    }}
                  >
                    View Guide
                  </Button>
                </Stack>
              </CardContent>
            </Card>
          </Grid>
        )}

        {/* Daily Usage Chart */}
        <Grid size={{ xs: 12 }}>
          <DailyUsageChart />
        </Grid>
      </Grid>
    </Box>
  );
}
