"use client";

import { useEffect, useRef } from "react";
import { usePathname, useRouter } from "next/navigation";
import {
  SESSION_COOKIE_NAME,
  AUTH_TOKEN_COOKIE_NAME,
  USER_DATA_COOKIE_NAME,
  CSRF_COOKIE_NAME,
} from "@/services/apiClient/baseApiClient";
import { isAuthRoute } from "@/constants/routes";

/**
 * Client-side component that validates authentication state and prevents redirect loops
 *
 * This component handles edge cases where:
 * 1. Cookies exist but are invalid after deployment
 * 2. Middleware let through requests with stale cookies
 * 3. Server-side cookie clearing failed
 *
 * It aggressively clears stale auth cookies on auth pages to prevent redirect loops
 */
export default function AuthStateValidator() {
  const pathname = usePathname();
  const router = useRouter();
  const hasRunRef = useRef(false);

  useEffect(() => {
    // Only run once per mount
    if (hasRunRef.current) return;
    hasRunRef.current = true;

    // Only run in browser
    if (typeof document === "undefined") return;

    // Check if we're on an auth page
    if (!isAuthRoute(pathname)) return;

    // Check if browser has auth cookies
    const hasBrowserSessionCookie = document.cookie.includes(`${SESSION_COOKIE_NAME}=`);
    const hasBrowserAuthToken = document.cookie.includes(`${AUTH_TOKEN_COOKIE_NAME}=`);
    const hasBrowserCookies = hasBrowserSessionCookie || hasBrowserAuthToken;

    // If we're on an auth page with cookies, they might be stale
    // Clear them proactively to prevent redirect loops
    if (hasBrowserCookies) {
      console.log("Detected stale cookies on auth page - clearing to prevent redirect loops");

      const deleteCookie = (name: string) => {
        // Try multiple deletion strategies to ensure cookies are cleared
        // across different domain configurations
        const deletionConfigs = [
          `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`,
          `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/; domain=${window.location.hostname};`,
          `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/; domain=.${window.location.hostname};`,
        ];

        deletionConfigs.forEach((config) => {
          document.cookie = config;
        });
      };

      deleteCookie(SESSION_COOKIE_NAME);
      deleteCookie(AUTH_TOKEN_COOKIE_NAME);
      deleteCookie(CSRF_COOKIE_NAME);
      deleteCookie(USER_DATA_COOKIE_NAME);

      // Force a page refresh to ensure clean state
      setTimeout(() => {
        router.refresh();
      }, 100);
    }
  }, [pathname, router]);

  return null; // This component doesn't render anything
}
