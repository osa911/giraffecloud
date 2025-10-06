import axios, { AxiosInstance, AxiosResponse, AxiosError } from "axios";
import toast from "react-hot-toast";
import baseApiClient, { BaseApiClientParams, CSRF_COOKIE_NAME, APIResponse } from "./baseApiClient";
import { ROUTES, isAuthRoute } from "@/constants/routes";

const baseURL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export const axiosClient: AxiosInstance = axios.create({
  baseURL,
  headers: {
    "Content-Type": "application/json",
    Accept: "application/json",
  },
  withCredentials: true, // Enable sending cookies with requests
});

// Utility to read a cookie by name
function getCookie(name: string): string | null {
  const value = `; ${document.cookie}`;
  const parts = value.split(`; ${name}=`);
  if (parts.length === 2) return parts.pop()!.split(";").shift()!;
  return null;
}

// Utility to delete a cookie by name
function deleteCookie(name: string): void {
  document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
}

/**
 * Clear all auth cookies (CLIENT-SIDE ONLY)
 *
 * This is the client-side implementation for browser-based cookie cleanup.
 * Used by the axios error interceptor when receiving 401/403 responses.
 *
 * NOTE: There's a similar clearAllAuthCookies() in auth.actions.ts for server-side.
 * They can't be shared because:
 * - This uses document.cookie (browser API)
 * - Server uses Next.js cookies() API
 * - Server actions can't be imported in client code
 *
 * If you add/remove cookies, update BOTH implementations!
 */
function clearAuthCookies(): void {
  deleteCookie("session");
  deleteCookie("auth_token");
  deleteCookie(CSRF_COOKIE_NAME);
  deleteCookie("user_data");
}

// Response interceptor for error handling
axiosClient.interceptors.response.use(
  (response: AxiosResponse) => {
    if (process.env.NODE_ENV === "development") {
      console.debug("API Response:", response.status, response.config.url);
    }
    return response;
  },
  (error: AxiosError<APIResponse<unknown>>) => {
    // Log the error in development only
    if (process.env.NODE_ENV === "development") {
      console.error("API Error:", error.response?.status, error.response?.data, error.config?.url);
    }

    // Handle CSRF errors
    const csrfErrorMessage = error.response?.data?.error?.message || "";

    if (
      error.response?.status === 403 &&
      typeof csrfErrorMessage === "string" &&
      csrfErrorMessage.includes("CSRF")
    ) {
      // Clear stale CSRF token
      clearAuthCookies();
      toast.error("Session expired. Please refresh the page and log in again.");

      // Redirect to login after a short delay
      setTimeout(() => {
        window.location.href = ROUTES.AUTH.LOGIN;
      }, 1500);

      return Promise.reject(new Error("CSRF token invalid"));
    }

    // Handle unauthorized access
    if (error.response?.status === 401) {
      // Clear stale auth cookies
      clearAuthCookies();

      // Don't redirect if we're already on an auth page or home page
      const currentPath = window.location.pathname;
      const isAuthOrHomePage = isAuthRoute(currentPath) || currentPath === ROUTES.HOME;

      if (!isAuthOrHomePage) {
        console.log("Session expired - redirecting to login page");
        toast.error("Session expired. Please log in again.");
        window.location.href = ROUTES.AUTH.LOGIN;
      }
    }

    // Show error toast for other errors
    if (error.response?.status !== 401 && error.response?.status !== 403) {
      const errorMessage = error.response?.data?.error?.message || "An error occurred";
      toast.error(errorMessage);
    }

    return Promise.reject(error);
  },
);

// Create and export the client API client
const clientApi = (params?: BaseApiClientParams) => {
  return baseApiClient(axiosClient, {
    ...params,
    csrfConfig: {
      getCsrfToken: () => {
        const token = getCookie(CSRF_COOKIE_NAME);
        return token || undefined;
      },
      onMissingToken: (method, url) => {
        if (process.env.NODE_ENV === "development") {
          console.warn("CSRF token missing for unsafe method:", method, url);
        }
      },
    },
  });
};

export default clientApi;
