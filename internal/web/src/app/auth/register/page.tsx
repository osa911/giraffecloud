import { redirect } from "next/navigation";
import { getAuthUser } from "@/lib/actions/auth.actions";
import { ROUTES } from "@/constants/routes";
import RegisterPage from "@/components/auth/register/RegisterPage";

// Server component
export default async function RegisterServerPage() {
  // If user is already authenticated, redirect to dashboard
  const user = await getAuthUser({ redirect: false });
  if (user) {
    redirect(ROUTES.DASHBOARD.HOME);
  }

  return <RegisterPage />;
}
