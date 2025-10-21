import { redirect } from "next/navigation";
import { getAuthUser } from "@/lib/actions/auth.actions";
import { ROUTES } from "@/constants/routes";
import RegisterPage from "@/components/auth/register/RegisterPage";

// Force dynamic rendering (uses cookies for auth)
export const dynamic = "force-dynamic";

// Server component
export default async function RegisterServerPage() {
  // If user is already authenticated, redirect to dashboard
  // Use updateCache: false since page components can't modify cookies in Next.js 15
  const user = await getAuthUser({ redirect: false, updateCache: false });
  if (user) {
    redirect(ROUTES.DASHBOARD.HOME);
  }

  return <RegisterPage />;
}
