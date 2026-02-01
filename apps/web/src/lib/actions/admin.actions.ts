"use server";

import serverApi from "@/services/apiClient/serverApiClient";
import { getAuthUser } from "./auth.actions";
import {
  VersionConfigsResponse,
  UpdateVersionConfigRequest,
  UpdateVersionConfigResponse,
  AdminUsersResponse,
  AdminUser,
  AdminApiError,
} from "./admin.types";

// Version Config Actions

export async function getVersionConfigs(): Promise<VersionConfigsResponse> {
  try {
    await getAuthUser();
    return await serverApi().get<VersionConfigsResponse>("/admin/version/configs");
  } catch (error) {
    const apiError = error as AdminApiError;
    console.error("Error fetching version configs:", apiError);
    throw apiError;
  }
}

export async function updateVersionConfig(
  data: UpdateVersionConfigRequest
): Promise<UpdateVersionConfigResponse> {
  try {
    await getAuthUser();
    return await serverApi().post<UpdateVersionConfigResponse>(
      "/admin/version/update",
      data
    );
  } catch (error) {
    const apiError = error as AdminApiError;
    console.error("Error updating version config:", apiError);
    throw apiError;
  }
}

// User Management Actions (Admin only)

export async function getAdminUsers(
  page = 1,
  pageSize = 10
): Promise<AdminUsersResponse> {
  try {
    await getAuthUser();
    return await serverApi().get<AdminUsersResponse>(
      `/admin/users?page=${page}&pageSize=${pageSize}`
    );
  } catch (error) {
    const apiError = error as AdminApiError;
    console.error("Error fetching admin users:", apiError);
    throw apiError;
  }
}

export async function getAdminUser(id: number): Promise<AdminUser> {
  try {
    await getAuthUser();
    return await serverApi().get<AdminUser>(`/admin/users/${id}`);
  } catch (error) {
    const apiError = error as AdminApiError;
    console.error("Error fetching admin user:", apiError);
    throw apiError;
  }
}

export async function updateAdminUser(
  id: number,
  data: { name?: string; email?: string; is_active?: boolean }
): Promise<AdminUser> {
  try {
    await getAuthUser();
    return await serverApi().put<AdminUser>(`/admin/users/${id}`, data);
  } catch (error) {
    const apiError = error as AdminApiError;
    console.error("Error updating admin user:", apiError);
    throw apiError;
  }
}

export async function deleteAdminUser(id: number): Promise<void> {
  try {
    await getAuthUser();
    await serverApi().delete(`/admin/users/${id}`);
  } catch (error) {
    const apiError = error as AdminApiError;
    console.error("Error deleting admin user:", apiError);
    throw apiError;
  }
}
