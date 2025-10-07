"use client";

import { useEffect } from "react";
import {
  SESSION_COOKIE_NAME,
  AUTH_TOKEN_COOKIE_NAME,
  USER_DATA_COOKIE_NAME,
  CSRF_COOKIE_NAME,
} from "@/services/apiClient/baseApiClient";

/**
 * Client-side component that validates cookie consistency
 * Clears browser cookies if they're inconsistent with server state
 *
 * This handles the case where server-side 401 cleared server cookies
 * but browser still has stale cookies
 */
export default function CookieValidator({ hasServerAuth }: { hasServerAuth: boolean }) {
  useEffect(() => {
    // Only run in browser
    if (typeof document === "undefined") return;

    // Check if browser has auth cookies
    const hasBrowserSessionCookie = document.cookie.includes(`${SESSION_COOKIE_NAME}=`);
    const hasBrowserAuthToken = document.cookie.includes(`${AUTH_TOKEN_COOKIE_NAME}=`);
    const hasBrowserCookies = hasBrowserSessionCookie || hasBrowserAuthToken;

    // If server says no auth but browser has cookies → clear them
    if (!hasServerAuth && hasBrowserCookies) {
      console.log("Clearing stale browser cookies (server auth expired)");

      // Delete cookies
      const deleteCookie = (name: string) => {
        document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
      };

      deleteCookie(SESSION_COOKIE_NAME);
      deleteCookie(AUTH_TOKEN_COOKIE_NAME);
      deleteCookie(CSRF_COOKIE_NAME);
      deleteCookie(USER_DATA_COOKIE_NAME);
    }
  }, [hasServerAuth]);

  return null; // This component doesn't render anything
}
