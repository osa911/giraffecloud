import { AxiosInstance, AxiosRequestConfig } from "axios";

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

// This interface defines the minimal contract that matches the axios interface we need
export type HttpClient = Pick<AxiosInstance, "get" | "post" | "put" | "delete">;

export type BaseApiClientParams = {
  prefix?: string;
  version?: string;
};

// The base client factory takes an HTTP client instance and creates an API client from it
const baseApiClient = (
  httpClient: HttpClient,
  params?: BaseApiClientParams
) => {
  const { prefix = "api", version = "v1" } = params || {};
  const baseURL = `/${prefix}/${version}`;

  return {
    get: async <T>(
      endpoint: string,
      config?: AxiosRequestConfig
    ): Promise<T> => {
      const url = `${baseURL}${endpoint}`;
      const response = await httpClient.get<APIResponse<T>>(url, config);

      // Return just the data from the response envelope
      return response.data.data as T;
    },

    post: async <T>(
      endpoint: string,
      data?: any,
      config?: AxiosRequestConfig
    ): Promise<T> => {
      const url = `${baseURL}${endpoint}`;
      const response = await httpClient.post<APIResponse<T>>(url, data, config);

      // Return just the data from the response envelope
      return response.data.data as T;
    },

    put: async <T>(
      endpoint: string,
      data?: any,
      config?: AxiosRequestConfig
    ): Promise<T> => {
      const url = `${baseURL}${endpoint}`;
      const response = await httpClient.put<APIResponse<T>>(url, data, config);

      // Return just the data from the response envelope
      return response.data.data as T;
    },

    delete: async <T>(
      endpoint: string,
      config?: AxiosRequestConfig
    ): Promise<T> => {
      const url = `${baseURL}${endpoint}`;
      const response = await httpClient.delete<APIResponse<T>>(url, config);

      // Return just the data from the response envelope
      return response.data.data as T;
    },
  };
};

export default baseApiClient;
