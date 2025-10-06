import { redirect } from "next/navigation";
import { getAuthUser } from "@/lib/actions/auth.actions";
import { ROUTES } from "@/constants/routes";
import LoginPage from "@/components/auth/login/LoginPage";

// Server component
export default async function LoginServerPage() {
  // If user is already authenticated, redirect to dashboard
  const user = await getAuthUser({ redirect: false });
  if (user) {
    redirect(ROUTES.DASHBOARD.HOME);
  }

  return <LoginPage />;
}
