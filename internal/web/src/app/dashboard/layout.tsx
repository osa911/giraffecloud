import { requireAuth } from "@/services/authServerService";
import DashboardLayoutClient from "@/components/dashboard/DashboardLayout";

type DashboardLayoutProps = {
  children: React.ReactNode;
};

// Server component
async function DashboardLayout({ children }: DashboardLayoutProps) {
  const user = await requireAuth();

  return <DashboardLayoutClient user={user}>{children}</DashboardLayoutClient>;
}

export default DashboardLayout;
