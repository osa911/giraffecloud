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

// NOTE: No response interceptor for cookie handling
//
// Cookie forwarding is handled explicitly in Server Actions (login, register)
// where we have direct access to response headers and can properly set cookies.
//
// The interceptor approach doesn't work because:
// - Server Action responses don't automatically forward Set-Cookie headers to browser
// - We need to manually extract headers and set cookies using Next.js cookies() API
// - This must be done in the Server Action itself, not in an interceptor

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
