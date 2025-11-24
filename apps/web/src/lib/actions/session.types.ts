// Response types
export type SessionResponse = {
  id: number;
  lastUsed: string;
  ipAddress: string;
  createdAt: string;
};

export type ListSessionsResponse = {
  sessions: SessionResponse[];
};

// Action types
export type GetSessionsAction = () => Promise<ListSessionsResponse>;
export type RevokeSessionAction = (id: number) => Promise<void>;
export type RevokeAllSessionsAction = () => Promise<void>;
