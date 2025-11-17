import { redirect } from "next/navigation";
import { getAuthUser } from "@/lib/actions/auth.actions";
import { ROUTES } from "@/constants/routes";
import RegisterPage from "@/components/auth/register/RegisterPage";
import AuthStateValidator from "@/components/auth/AuthStateValidator";

// Force dynamic rendering (uses cookies for auth)
export const dynamic = "force-dynamic";

// Server component
export default async function RegisterServerPage() {
  // If user is already authenticated, redirect to dashboard
  // Use updateCache: true to clear invalid cookies and prevent redirect loops
  const user = await getAuthUser({ redirect: false, updateCache: true });
  if (user) {
    redirect(ROUTES.DASHBOARD.HOME);
  }

  return (
    <>
      <AuthStateValidator />
      <RegisterPage />
    </>
  );
}
