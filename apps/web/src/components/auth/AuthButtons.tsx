"use client";

import Link from "next/link";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/contexts/AuthProvider";
import { ROUTES } from "@/constants/routes";

interface AuthButtonsProps {
  className?: string;
}

export default function AuthButtons({ className }: AuthButtonsProps) {
  const { user, loading } = useAuth();

  console.log("user", user);
  console.log("loading", loading);
  if (loading) return null;

  if (user) {
    return (
      <Button asChild size="sm" className={className}>
        <Link href={ROUTES.DASHBOARD.HOME}>Go to Dashboard</Link>
      </Button>
    );
  }

  return (
    <div className={`flex items-center gap-2 ${className}`}>
      <Button asChild variant="ghost" size="sm">
        <Link href={ROUTES.AUTH.LOGIN}>Login</Link>
      </Button>
      <Button asChild size="sm">
        <Link href={ROUTES.AUTH.REGISTER}>Get Started</Link>
      </Button>
    </div>
  );
}
