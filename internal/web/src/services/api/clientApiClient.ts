import axios, {
  AxiosInstance,
  InternalAxiosRequestConfig,
  AxiosResponse,
} from "axios";
import toast from "react-hot-toast";
import baseApiClient, { BaseApiClientParams } from "../baseClientService";

const baseURL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export const axiosClient: AxiosInstance = axios.create({
  baseURL,
  headers: {
    "Content-Type": "application/json",
    Accept: "application/json",
  },
  withCredentials: true, // Enable sending cookies with requests
});

// Request interceptor to add auth token
axiosClient.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  // Initialize headers if not set
  config.headers = config.headers || {};

  // Debug the request
  console.debug("API Request:", {
    method: config.method,
    url: config.url,
    data: config.data
      ? JSON.stringify(config.data).substring(0, 100) + "..."
      : "(no data)",
    headers: Object.keys(config.headers || {}),
    withCredentials: config.withCredentials,
  });

  return config;
});

// Response interceptor for error handling
axiosClient.interceptors.response.use(
  (response: AxiosResponse) => {
    console.debug("API Response:", response.status, response.config.url);
    return response;
  },
  (error: any) => {
    // Log the error for debugging
    console.error(
      "API Error:",
      error.response?.status,
      error.response?.data,
      error.config?.url
    );

    // Handle unauthorized access
    if (error.response?.status === 401) {
      // Don't redirect if we're already on an auth page
      const currentPath = window.location.pathname;
      const isAuthPage =
        currentPath.includes("/login") ||
        currentPath.includes("/register") ||
        currentPath.includes("/test-register") ||
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
const apiClient = (params?: BaseApiClientParams) => {
  return baseApiClient(axiosClient, params);
};

export default apiClient;
