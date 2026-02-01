// Admin Types

// Version configuration from backend
export type VersionConfig = {
  id: string;
  channel: string;
  platform: string;
  arch: string;
  latest_version: string;
  minimum_version: string;
  download_url: string;
  release_notes: string;
  auto_update_enabled: boolean;
  force_update: boolean;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

// Request to update version config
export type UpdateVersionConfigRequest = {
  channel: string;
  platform: string;
  arch: string;
  latest_version: string;
  minimum_version: string;
  download_url?: string;
  release_notes?: string;
  auto_update_enabled?: boolean;
  force_update?: boolean;
  metadata?: Record<string, unknown>;
};

// Response types
export type VersionConfigsResponse = {
  configs: VersionConfig[];
};

export type UpdateVersionConfigResponse = {
  message: string;
  config: VersionConfig;
};

// User list types (reusing from user.types.ts for admin context)
export type AdminUser = {
  id: number;
  email: string;
  name: string;
  role: string;
  is_active: boolean;
  last_login: string | null;
  last_login_ip: string | null;
  last_activity: string | null;
  created_at: string;
  updated_at: string;
};

export type AdminUsersResponse = {
  users: AdminUser[];
  total_count: number;
  page: number;
  page_size: number;
};

// Error type
export type AdminApiError = {
  message: string;
  code?: string;
  status?: number;
};
