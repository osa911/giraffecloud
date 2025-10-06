import { AxiosResponse, AxiosInstance, AxiosRequestConfig } from "axios";

// Constants for CSRF handling
export const CSRF_COOKIE_NAME = "csrf_token";
export const AUTH_TOKEN_COOKIE_NAME = "auth_token";
const CSRF_HEADER_NAME = "X-CSRF-Token";

// HTTP methods that require CSRF protection
const UNSAFE_METHODS = ["post", "put", "patch", "delete"] as const;

// Endpoints that should skip CSRF protection
const AUTH_ENDPOINTS = ["/auth/login", "/auth/register"] as const;

// Define the standard API response structure
export interface APIResponse<T> {
  success: boolean;
  data?: T;
  error?: {
    code: string;
    message: string;
    details?: unknown;
  };
}

// CSRF-related types
type CSRFConfig = {
  getCsrfToken: () => string | undefined | Promise<string | undefined>;
  shouldSkipCsrf?: (url: string) => boolean;
  onMissingToken?: (method: string, url?: string) => void;
};

// This interface defines the minimal contract that matches the axios interface we need
type HttpClient = Pick<AxiosInstance, "get" | "post" | "put" | "delete">;

export type BaseApiClientParams = {
  prefix?: string;
  version?: string;
  csrfConfig?: CSRFConfig;
};

// Utility function to check if a method requires CSRF token
const requiresCsrfToken = (method?: string): boolean => {
  return (
    !!method && UNSAFE_METHODS.includes(method.toLowerCase() as (typeof UNSAFE_METHODS)[number])
  );
};

// Utility function to check if an endpoint should skip CSRF
const isAuthEndpoint = (url?: string): boolean => {
  return !!url && AUTH_ENDPOINTS.some((endpoint) => url.endsWith(endpoint));
};

// The base client factory takes an HTTP client instance and creates an API client from it
const baseApiClient = (httpClient: HttpClient, params?: BaseApiClientParams) => {
  const { prefix = "api", version = "v1", csrfConfig } = params || {};
  const baseURL = `/${prefix}/${version}`;

  // Helper to add CSRF token to request config if needed
  const addCsrfToken = async (config: AxiosRequestConfig = {}): Promise<AxiosRequestConfig> => {
    if (!csrfConfig) return config;

    const { getCsrfToken, shouldSkipCsrf = isAuthEndpoint, onMissingToken } = csrfConfig;
    config.headers = config.headers || {};

    if (requiresCsrfToken(config.method) && !shouldSkipCsrf(config.url || "")) {
      const token = await getCsrfToken();
      if (token) {
        config.headers[CSRF_HEADER_NAME] = token;
      } else if (onMissingToken) {
        onMissingToken(config.method || "", config.url);
      }
    }

    return config;
  };

  // Helper to prepare URL and config
  const prepareRequest = async (
    endpoint: string,
    method: string,
    config: AxiosRequestConfig = {},
  ): Promise<{ url: string; config: AxiosRequestConfig }> => {
    const url = `${baseURL}${endpoint}`;
    config.method = method;
    config.url = url;
    return { url, config: await addCsrfToken(config) };
  };

  return {
    get: async <T>(endpoint: string, config: AxiosRequestConfig = {}): Promise<T> => {
      const { url, config: finalConfig } = await prepareRequest(endpoint, "get", config);
      const response = await httpClient.get<APIResponse<T>>(url, finalConfig);
      return response.data.data as T;
    },

    post: async <T>(
      endpoint: string,
      data?: unknown,
      config: AxiosRequestConfig = {},
    ): Promise<T> => {
      const { url, config: finalConfig } = await prepareRequest(endpoint, "post", config);
      const response = await httpClient.post<APIResponse<T>>(url, data, finalConfig);
      return response.data.data as T;
    },

    /**
     * Post with full response (includes headers)
     * Useful when you need access to Set-Cookie or other response headers
     */
    postRaw: async <T>(
      endpoint: string,
      data?: unknown,
      config: AxiosRequestConfig = {},
    ): Promise<AxiosResponse<APIResponse<T>>> => {
      const { url, config: finalConfig } = await prepareRequest(endpoint, "post", config);
      return await httpClient.post<APIResponse<T>>(url, data, finalConfig);
    },

    put: async <T>(
      endpoint: string,
      data?: unknown,
      config: AxiosRequestConfig = {},
    ): Promise<T> => {
      const { url, config: finalConfig } = await prepareRequest(endpoint, "put", config);
      const response = await httpClient.put<APIResponse<T>>(url, data, finalConfig);
      return response.data.data as T;
    },

    delete: async <T>(endpoint: string, config: AxiosRequestConfig = {}): Promise<T> => {
      const { url, config: finalConfig } = await prepareRequest(endpoint, "delete", config);
      const response = await httpClient.delete<APIResponse<T>>(url, finalConfig);
      return response.data.data as T;
    },
  };
};

export default baseApiClient;
