"use client";

import { useEffect, useRef } from "react";
import {
  SESSION_COOKIE_NAME,
  AUTH_TOKEN_COOKIE_NAME,
  USER_DATA_COOKIE_NAME,
  CSRF_COOKIE_NAME,
} from "@/services/apiClient/baseApiClient";

/**
 * Client-side component that clears stale auth cookies on auth pages
 *
 * This component only runs on login/register pages when the server has already
 * determined there's no valid session. It clears browser cookies to ensure
 * clean state and prevent redirect loops.
 *
 * Safety: Only clears cookies when explicitly mounted on auth pages where
 * the server has confirmed no valid user session exists.
 */
export default function StaleAuthCookieCleaner() {
  const hasRunRef = useRef(false);

  useEffect(() => {
    // Only run once per mount
    if (hasRunRef.current) return;
    hasRunRef.current = true;

    // Only run in browser
    if (typeof document === "undefined") return;

    // Check if browser has auth cookies
    const hasBrowserCookies =
      document.cookie.includes(`${SESSION_COOKIE_NAME}=`) ||
      document.cookie.includes(`${AUTH_TOKEN_COOKIE_NAME}=`);

    // If cookies exist, clear them
    // We know they're stale because this component only renders when server said no valid user
    if (hasBrowserCookies) {
      console.log("[StaleAuthCookieCleaner] Clearing stale auth cookies on auth page");
      console.log("[StaleAuthCookieCleaner] Current cookies:", document.cookie);

      const deleteCookie = (name: string) => {
        // Clear with multiple domain configurations to ensure deletion
        const configs = [
          `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`,
          `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/; domain=${window.location.hostname};`,
        ];

        if (window.location.hostname.includes(".")) {
          const rootDomain = window.location.hostname.split(".").slice(-2).join(".");
          configs.push(`${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/; domain=.${rootDomain};`);
        }

        configs.forEach((config) => {
          document.cookie = config;
        });
      };

      // Delete all auth-related cookies
      deleteCookie(SESSION_COOKIE_NAME);
      deleteCookie(AUTH_TOKEN_COOKIE_NAME);
      deleteCookie(CSRF_COOKIE_NAME);
      deleteCookie(USER_DATA_COOKIE_NAME);

      // Verify cookies were cleared
      setTimeout(() => {
        console.log("[StaleAuthCookieCleaner] Cookies after clearing:", document.cookie);
        const stillHasCookies =
          document.cookie.includes(`${SESSION_COOKIE_NAME}=`) ||
          document.cookie.includes(`${AUTH_TOKEN_COOKIE_NAME}=`);
        if (stillHasCookies) {
          console.warn("[StaleAuthCookieCleaner] WARNING: Cookies still present after clearing attempt");
        } else {
          console.log("[StaleAuthCookieCleaner] Successfully cleared all auth cookies");
        }
      }, 100);
    }
  }, []);

  return null; // This component doesn't render anything
}

