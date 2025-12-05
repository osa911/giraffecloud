// Tunnel response (no token)
export enum DnsPropagationStatus {
  VERIFIED = 'verified',
  PENDING_DNS = 'pending_dns',
}

export interface Tunnel {
  id: number;
  domain: string;
  target_port: number;
  is_enabled: boolean;
  dns_propagation_status: DnsPropagationStatus;
  client_ip?: string;
  created_at: string;
  updated_at: string;
}

// Tunnel create response (includes token)
export interface TunnelCreateResponse {
  id: number;
  domain: string;
  token: string;
  target_port: number;
  is_enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface TunnelFormData {
  domain: string;
  target_port: number;
  is_enabled?: boolean;
}

export interface FreeSubdomainResponse {
  domain: string;
  available: boolean;
}

export interface UsageData {
  day: {
    period_start: string;
    bytes_in: number;
    bytes_out: number;
    requests: number;
  };
  month: {
    used_bytes: number;
    limit_bytes: number;
    decision: string;
  };
}

export interface DashboardStats {
  totalTunnels: number;
  enabledTunnels: number;
  totalTraffic: number;
}

export interface DailyUsageEntry {
  date: string;
  bytes_in: number;
  bytes_out: number;
  requests: number;
  total: number;
}

export interface DailyUsageHistory {
  history: DailyUsageEntry[];
  days: number;
}
