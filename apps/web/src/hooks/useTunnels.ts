import useSWR from "swr";
import type { Tunnel, FreeSubdomainResponse } from "@/types/tunnel";
import clientApi from "@/services/apiClient/clientApiClient";

const fetcher = async (url: string) => {
  return clientApi().get<Tunnel[]>(url);
};

export function useTunnels() {
  const { data, error, isLoading, mutate } = useSWR<Tunnel[]>("/tunnels", fetcher, {
    // Log errors but don't show toast - clientApiClient already handles error toasts
    onError: (err) => {
      console.error("Failed to fetch tunnels:", err);
    },
  });

  // Sort tunnels by ID (ascending) to maintain consistent order
  // Return empty array if data is undefined/null to prevent white screens
  const sortedTunnels = data ? [...data].sort((a, b) => a.id - b.id) : [];

  return {
    tunnels: sortedTunnels,
    isLoading,
    isError: error,
    mutate,
  };
}

export async function getFreeSubdomain(): Promise<FreeSubdomainResponse> {
  return clientApi().get<FreeSubdomainResponse>("/tunnels/free");
}
