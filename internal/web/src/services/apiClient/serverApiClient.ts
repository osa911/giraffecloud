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

// Add response interceptor to handle cookies
serverAxios.interceptors.response.use(async (response) => {
  const cookieStore = await cookies();

  // Get Set-Cookie headers from Go backend
  const setCookieHeaders = response.headers["set-cookie"];
  if (setCookieHeaders) {
    // Parse and set each cookie
    setCookieHeaders.forEach((cookieStr) => {
      const [nameValue, ...options] = cookieStr.split("; ");
      const [name, value] = nameValue.split("=");

      const cookieOptions: any = {};
      options.forEach((opt) => {
        const [key, val = true] = opt.toLowerCase().split("=");
        switch (key) {
          case "path":
            cookieOptions.path = val;
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

      // Set the cookie in the browser
      cookieStore.set(name, value, cookieOptions);
    });
  }

  return response;
});

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
