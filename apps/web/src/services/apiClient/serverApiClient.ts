"use server";

import { cookies } from "next/headers";
import axios, { AxiosError } from "axios";
import baseApiClient, {
  BaseApiClientParams,
  CSRF_COOKIE_NAME,
  SESSION_COOKIE_NAME,
  AUTH_TOKEN_COOKIE_NAME,
  USER_DATA_COOKIE_NAME,
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

// Add response interceptor for error handling
serverAxios.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    // Handle 401 Unauthorized - clear stale cookies
    if (error.response?.status === 401) {
      try {
        const cookieStore = await cookies();
        cookieStore.delete(SESSION_COOKIE_NAME);
        cookieStore.delete(AUTH_TOKEN_COOKIE_NAME);
        cookieStore.delete(CSRF_COOKIE_NAME);
        cookieStore.delete(USER_DATA_COOKIE_NAME);
      } catch (cookieError) {
        // Cookie clearing failed - likely during SSR/static generation
        // This is expected and safe to ignore - the client-side will handle it
        if (process.env.NODE_ENV === "development") {
          const errorMessage =
            cookieError instanceof Error ? cookieError.message : String(cookieError);
          if (!errorMessage.includes("Server Action or Route Handler")) {
            console.warn("Failed to clear cookies on 401:", errorMessage);
          }
        }
      }
    }
    return Promise.reject(error);
  },
);

// NOTE: Set-Cookie header forwarding is handled explicitly in Server Actions (login, register)
// where we have direct access to response headers and can properly set cookies.
// This must be done in the Server Action itself, not in an interceptor.

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
