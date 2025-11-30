import clientApi from "@/services/apiClient/clientApiClient";

export const fetcher = async <T>(url: string): Promise<T> => {
  const api = clientApi();
  return api.get<T>(url);
};
