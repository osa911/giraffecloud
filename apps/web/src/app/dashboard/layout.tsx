import { getAuthUser } from "@/lib/actions/auth.actions";
import DashboardLayout from "@/components/dashboard/DashboardLayout";

type DashboardServerLayoutProps = {
  children: React.ReactNode;
};

// Server component
async function DashboardServerLayout({ children }: DashboardServerLayoutProps) {
  const user = await getAuthUser({ redirect: true, updateCache: false });
  return <DashboardLayout user={user}>{children}</DashboardLayout>;
}

export default DashboardServerLayout;
