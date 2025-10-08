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
  activeTunnels: number;
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
