"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthProvider";
import { ROUTES } from "@/constants/routes";

/**
 * Client-side component that redirects logged-in users from home to dashboard
 * This keeps the home page static for SEO while handling auth redirects client-side
 */
export default function HomeRedirectHandler() {
  const { user } = useAuth();
  const router = useRouter();

  useEffect(() => {
    // Redirect if user is authenticated
    if (user) {
      router.push(ROUTES.DASHBOARD.HOME);
    }
  }, [user, router]);

  // This component doesn't render anything
  return null;
}
