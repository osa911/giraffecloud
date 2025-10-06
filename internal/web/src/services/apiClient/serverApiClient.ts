"use server";

import { cookies } from "next/headers";
import axios from "axios";
import baseApiClient, {
  BaseApiClientParams,
  CSRF_COOKIE_NAME,
} from "@/services/apiClient/baseApiClient";

const serverAxios = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080",
  headers: {
    "Content-Type": "application/json",
    Accept: "application/json",
  },
});

// Add request interceptor to include cookies
serverAxios.interceptors.request.use(async (config) => {
  const cookieStore = await cookies();
  config.headers = config.headers || {};

  // Add all cookies
  config.headers.Cookie = cookieStore;
  return config;
});

// Add response interceptor to forward Set-Cookie headers from Go backend
serverAxios.interceptors.response.use(
  async (response) => {
    // Get Set-Cookie headers from Go backend
    const setCookieHeaders = response.headers["set-cookie"];
    if (!setCookieHeaders) {
      return response;
    }

    // Try to set cookies - this only works in Server Actions, not SSR
    try {
      const cookieStore = await cookies();

      // Parse and set each cookie
      setCookieHeaders.forEach((cookieStr) => {
        const [nameValue, ...options] = cookieStr.split("; ");
        const [name, value] = nameValue.split("=");

        const cookieOptions: Record<string, unknown> = {};
        options.forEach((opt) => {
          const [key, val = true] = opt.toLowerCase().split("=");
          switch (key) {
            case "path":
              cookieOptions.path = val;
              break;
            case "domain":
              cookieOptions.domain = val;
              break;
            case "max-age":
              cookieOptions.maxAge = parseInt(val as string);
              break;
            case "secure":
              cookieOptions.secure = true;
              break;
            case "httponly":
              cookieOptions.httpOnly = true;
              break;
            case "samesite":
              cookieOptions.sameSite = val;
              break;
          }
        });

        // Set the cookie (forwarding from Go backend to browser)
        cookieStore.set(name, value, cookieOptions);
      });
    } catch (error) {
      // Cookie modification failed - likely called during SSR/page rendering
      // This is expected and safe to ignore (SSR shouldn't be setting cookies anyway)
      // Important: We still return the response successfully - the data is what matters
      if (process.env.NODE_ENV === "development") {
        const errorMessage = error instanceof Error ? error.message : String(error);
        // Only warn about unexpected errors, not the expected SSR case
        if (!errorMessage.includes("Server Action or Route Handler")) {
          console.warn("Failed to set cookies from backend:", errorMessage);
        }
      }
    }

    // Always return response, even if cookie setting failed
    return response;
  },
  async (error) => {
    // Don't try to clear cookies here - it will fail during SSR
    // Cookie cleanup happens in Server Actions (getAuthUser, logout)
    return Promise.reject(error);
  },
);

// Create and export the server API client
const serverApi = (params?: BaseApiClientParams) => {
  return baseApiClient(serverAxios, {
    ...params,
    csrfConfig: {
      getCsrfToken: async () => {
        const cookieStore = await cookies();
        return cookieStore.get(CSRF_COOKIE_NAME)?.value;
      },
      onMissingToken: (method, url) => {
        if (process.env.NODE_ENV === "development") {
          console.warn("CSRF token missing for unsafe method:", method, url);
        }
      },
    },
  });
};

export default serverApi;
