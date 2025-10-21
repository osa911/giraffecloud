import useSWR from "swr";
import type { Tunnel, FreeSubdomainResponse } from "@/types/tunnel";
import clientApi from "@/services/apiClient/clientApiClient";

const fetcher = async (url: string) => {
  return clientApi().get<Tunnel[]>(url);
};

export function useTunnels() {
  const { data, error, isLoading, mutate } = useSWR<Tunnel[]>("/tunnels", fetcher);

  return {
    tunnels: data,
    isLoading,
    isError: error,
    mutate,
  };
}

export async function getFreeSubdomain(): Promise<FreeSubdomainResponse> {
  return clientApi().get<FreeSubdomainResponse>("/tunnels/free");
}
