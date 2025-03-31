export interface Tunnel {
  id: string;
  name: string;
  localPort: number;
  remotePort?: number;
  protocol: "http" | "https" | "tcp" | "udp";
  publicUrl?: string;
  status: "online" | "offline" | "error";
  createdAt: string;
  updatedAt: string;
}

export interface ApiResponse<T> {
  data: T;
  message?: string;
  success: boolean;
}

export interface ApiError {
  message: string;
  statusCode: number;
  error: string;
}
