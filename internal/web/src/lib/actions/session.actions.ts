"use server";

import serverApi from "@/services/apiClient/serverApiClient";
import { getAuthUser } from "./auth.actions";
import { GetSessionsAction, RevokeSessionAction, RevokeAllSessionsAction } from "./session.types";

// Session Management Actions
export const getSessions: GetSessionsAction = async () => {
  await getAuthUser();
  return serverApi().get("/sessions");
};

export const revokeSession: RevokeSessionAction = async (id) => {
  await getAuthUser();
  return serverApi().delete(`/sessions/${id}`);
};

export const revokeAllSessions: RevokeAllSessionsAction = async () => {
  await getAuthUser();
  return serverApi().delete("/sessions");
};
