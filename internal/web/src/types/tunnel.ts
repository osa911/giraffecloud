export interface Tunnel {
  id: number;
  domain: string;
  target_port: number;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface TunnelFormData {
  domain: string;
  target_port: number;
  is_active: boolean;
}
