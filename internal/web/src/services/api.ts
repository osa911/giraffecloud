import axios, {
  AxiosInstance,
  InternalAxiosRequestConfig,
  AxiosResponse,
  AxiosRequestConfig,
} from "axios";
import toast from "react-hot-toast";

const baseURL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// Define the standard API response structure
export interface APIResponse<T> {
  success: boolean;
  data?: T;
  error?: {
    code: string;
    message: string;
    details?: any;
  };
}

export const axiosClient: AxiosInstance = axios.create({
  baseURL,
  headers: {
    "Content-Type": "application/json",
    Accept: "application/json",
  },
  withCredentials: false,
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
  });

  if (typeof window !== "undefined") {
    // Get token from localStorage (support both firebase_token and token for backward compatibility)
    const token = localStorage.getItem("firebase_token");
    if (token) {
      console.debug("Using token authentication, token length:", token.length);
      config.headers.Authorization = `Bearer ${token}`;
    } else {
      console.debug("No token found in localStorage");
    }
  }
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

    if (error.response?.status === 401) {
      // Handle unauthorized access
      localStorage.removeItem("firebase_token");

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

type ApiClientParams = {
  prefix?: string;
  version?: string;
};
export const apiClient = (params?: ApiClientParams) => {
  const { prefix = "api", version = "v1" } = params || {};
  const baseURL = `/${prefix}/${version}`;

  return {
    get: async <T>(
      endpoint: string,
      config?: AxiosRequestConfig
    ): Promise<T> => {
      const url = `${baseURL}${endpoint}`;
      const response = await axiosClient.get<APIResponse<T>>(url, config);

      // Return just the data from the response envelope
      return response.data.data as T;
    },

    post: async <T>(
      endpoint: string,
      data?: any,
      config?: AxiosRequestConfig
    ): Promise<T> => {
      const url = `${baseURL}${endpoint}`;
      const response = await axiosClient.post<APIResponse<T>>(
        url,
        data,
        config
      );

      // Return just the data from the response envelope
      return response.data.data as T;
    },

    put: async <T>(
      endpoint: string,
      data?: any,
      config?: AxiosRequestConfig
    ): Promise<T> => {
      const url = `${baseURL}${endpoint}`;
      const response = await axiosClient.put<APIResponse<T>>(url, data, config);

      // Return just the data from the response envelope
      return response.data.data as T;
    },

    delete: async <T>(
      endpoint: string,
      config?: AxiosRequestConfig
    ): Promise<T> => {
      const url = `${baseURL}${endpoint}`;
      const response = await axiosClient.delete<APIResponse<T>>(url, config);

      // Return just the data from the response envelope
      return response.data.data as T;
    },
  };
};
