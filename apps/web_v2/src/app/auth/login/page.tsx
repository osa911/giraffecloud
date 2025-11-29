import { redirect } from "next/navigation";
import { getAuthUser } from "@/lib/actions/auth.actions";
import { ROUTES } from "@/constants/routes";
import LoginPage from "@/components/auth/login/LoginPage";
import StaleAuthCookieCleaner from "@/components/auth/StaleAuthCookieCleaner";

// Force dynamic rendering (uses cookies for auth)
export const dynamic = "force-dynamic";

// Server component
export default async function LoginServerPage() {
  // If user is already authenticated, redirect to dashboard
  const user = await getAuthUser({ redirect: false, updateCache: false });
  if (user) {
    redirect(ROUTES.DASHBOARD.HOME);
  }

  return (
    <>
      <StaleAuthCookieCleaner />
      <LoginPage />
    </>
  );
}
