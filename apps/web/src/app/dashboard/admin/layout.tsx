import { redirect } from "next/navigation";
import { getAuthUser } from "@/lib/actions/auth.actions";
import { ROUTES } from "@/constants/routes";

type AdminLayoutProps = {
  children: React.ReactNode;
};

export default async function AdminLayout({ children }: AdminLayoutProps) {
  const user = await getAuthUser({ redirect: true, updateCache: false });

  // Check if user has admin role
  if (user?.role !== "admin") {
    // Redirect non-admins to dashboard
    redirect(ROUTES.DASHBOARD.HOME);
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Admin Dashboard</h1>
          <p className="text-muted-foreground">
            Manage version configs, users, and system settings.
          </p>
        </div>
        <div className="px-3 py-1 rounded-full bg-yellow-500/10 text-yellow-600 dark:text-yellow-400 text-xs font-medium border border-yellow-500/20">
          Admin Only
        </div>
      </div>
      {children}
    </div>
  );
}
