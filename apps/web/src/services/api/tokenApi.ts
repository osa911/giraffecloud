import clientApi from "@/services/apiClient/clientApiClient";

export interface Token {
  id: string;
  name: string;
  created_at: string;
  last_used_at: string;
  expires_at: string;
}

export interface CreateTokenResponse {
  token: string;
  id: string;
  name: string;
  created_at: string;
  expires_at: string;
}

export const getTokensList = async (): Promise<Token[]> => {
  return await clientApi().get<Token[]>("/tokens");
};

export const createToken = async (name: string): Promise<CreateTokenResponse> => {
  return await clientApi().post<CreateTokenResponse>("/tokens", {
    name,
  });
};

export const revokeToken = async (id: string): Promise<void> => {
  await clientApi().delete(`/tokens/${id}`);
};
