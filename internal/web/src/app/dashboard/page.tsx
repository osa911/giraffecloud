import { getAuthUser } from "@/lib/actions/auth.actions";
import DashboardPage from "@/components/dashboard/DashboardPage";
import serverApi from "@/services/apiClient/serverApiClient";
import type { Tunnel, UsageData, DashboardStats } from "@/types/tunnel";

// Server component
export default async function DashboardServerPage() {
  // check if user is authenticated
  await getAuthUser();
  const stats = await fetchDashboardStats();

  return <DashboardPage initialStats={stats} />;
}

// Server-side data fetching function
async function fetchDashboardStats(): Promise<DashboardStats> {
  try {
    const api = serverApi();

    // Fetch tunnels data
    const tunnelsResponse = await api.get<Tunnel[]>("/tunnels");
    const tunnels = tunnelsResponse || [];

    // Calculate tunnel statistics
    const totalTunnels = tunnels.length;
    const activeTunnels = tunnels.filter((tunnel) => tunnel.is_active).length;

    // Fetch usage data for traffic statistics
    let totalTraffic = 0;
    try {
      const usageResponse = await api.get<UsageData>("/usage/summary");
      if (usageResponse?.month) {
        // Use monthly total traffic (current billing period)
        totalTraffic = usageResponse.month.used_bytes;
      }
    } catch (usageError) {
      console.warn("Could not fetch usage data:", usageError);
      // Continue with 0 traffic if usage API fails
    }

    return {
      totalTunnels,
      activeTunnels,
      totalTraffic,
    };
  } catch (error) {
    console.error("Error fetching dashboard stats:", error);
    // Return default values if API calls fail
    return {
      totalTunnels: 0,
      activeTunnels: 0,
      totalTraffic: 0,
    };
  }
}
