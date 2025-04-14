import { getAuthUser } from "@/lib/actions/auth.actions";
import DashboardPage from "@/components/dashboard/DashboardPage";

// Server component
export default async function DashboardServerPage() {
  const user = await getAuthUser();
  const stats = await fetchDashboardStats(user.id);

  return <DashboardPage initialStats={stats} />;
}

// Server-side data fetching function
async function fetchDashboardStats(userId: number) {
  // You would typically fetch this from your API
  // For now, we'll return mock data
  return {
    totalTunnels: 0,
    activeTunnels: 0,
    totalTraffic: 0,
  };
}
