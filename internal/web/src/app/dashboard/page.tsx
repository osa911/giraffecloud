import { requireAuth } from "@/services/authServerService";
import Dashboard from "@/components/dashboard/Dashboard";

// Server component
export default async function DashboardPage() {
  const user = await requireAuth();
  const stats = await fetchDashboardStats(user.id);

  return <Dashboard initialStats={stats} />;
}

// Server-side data fetching function
async function fetchDashboardStats(userId: number) {
  // You would typically fetch this from your API
  // For now, we'll return mock data
  return {
    totalTunnels: 5,
    activeTunnels: 2,
    totalTraffic: 1.5,
  };
}
