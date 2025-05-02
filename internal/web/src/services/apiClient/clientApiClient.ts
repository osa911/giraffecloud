import axios, {
  AxiosInstance,
  InternalAxiosRequestConfig,
  AxiosResponse,
} from "axios";
import toast from "react-hot-toast";
import baseApiClient, {
  BaseApiClientParams,
  CSRF_COOKIE_NAME,
} from "./baseApiClient";

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

// Response interceptor for error handling
axiosClient.interceptors.response.use(
  (response: AxiosResponse) => {
    if (process.env.NODE_ENV === "development") {
      console.debug("API Response:", response.status, response.config.url);
    }
    return response;
  },
  (error: any) => {
    // Log the error in development only
    if (process.env.NODE_ENV === "development") {
      console.error(
        "API Error:",
        error.response?.status,
        error.response?.data,
        error.config?.url
      );
    }

    // Handle CSRF errors
    if (
      error.response?.status === 403 &&
      error.response?.data?.error?.message?.includes("CSRF")
    ) {
      toast.error("Security token expired. Please refresh the page.");
      return Promise.reject(new Error("CSRF token invalid"));
    }

    // Handle unauthorized access
    if (error.response?.status === 401) {
      // Don't redirect if we're already on an auth page
      const currentPath = window.location.pathname;
      const isAuthPage =
        currentPath.includes("/auth/login") ||
        currentPath.includes("/auth/register") ||
        currentPath === "/";

      if (!isAuthPage) {
        console.log("Unauthorized - redirecting to login page");
        window.location.href = "/auth/login";
      }
    }

    // Show error toast - adapted for new error format
    const errorMessage =
      error.response?.data?.error?.message ||
      error.response?.data?.message ||
      "An error occurred";
    toast.error(errorMessage);

    return Promise.reject(error);
  }
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
