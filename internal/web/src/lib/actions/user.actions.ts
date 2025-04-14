"use server";

import serverApi from "@/services/apiClient/serverApiClient";
import { getAuthUser } from "./auth.actions";
import {
  UserResponse,
  ListUsersResponse,
  UpdateProfileRequest,
  UpdateUserRequest,
  GetProfileAction,
  UpdateProfileAction,
  DeleteProfileAction,
  ListUsersAction,
  GetUserAction,
  UpdateUserAction,
  DeleteUserAction,
  FormState,
  UpdateProfileFormAction,
  ApiError,
} from "./user.types";

// Profile Actions
export const getProfile: GetProfileAction = async () => {
  try {
    await getAuthUser();
    return await serverApi().get<UserResponse>("/user/profile");
  } catch (error) {
    const apiError = error as ApiError;
    console.error("Error fetching profile:", apiError);
    throw apiError;
  }
};

export const updateProfile: UpdateProfileAction = async (data) => {
  try {
    await getAuthUser();
    return await serverApi().put<UserResponse>("/user/profile", data);
  } catch (error) {
    const apiError = error as ApiError;
    console.error("Error updating profile:", apiError);
    throw apiError;
  }
};

export const deleteProfile: DeleteProfileAction = async () => {
  try {
    await getAuthUser();
    await serverApi().delete("/user/profile");
  } catch (error) {
    const apiError = error as ApiError;
    console.error("Error deleting profile:", apiError);
    throw apiError;
  }
};

// User Management Actions
export const listUsers: ListUsersAction = async (page = 1, pageSize = 10) => {
  try {
    await getAuthUser();
    return await serverApi().get<ListUsersResponse>(
      `/users?page=${page}&pageSize=${pageSize}`
    );
  } catch (error) {
    const apiError = error as ApiError;
    console.error("Error listing users:", apiError);
    throw apiError;
  }
};

export const getUser: GetUserAction = async (id) => {
  try {
    await getAuthUser();
    return await serverApi().get<UserResponse>(`/users/${id}`);
  } catch (error) {
    const apiError = error as ApiError;
    console.error("Error fetching user:", apiError);
    throw apiError;
  }
};

export const updateUser: UpdateUserAction = async (id, data) => {
  try {
    await getAuthUser();
    return await serverApi().put<UserResponse>(`/users/${id}`, data);
  } catch (error) {
    const apiError = error as ApiError;
    console.error("Error updating user:", apiError);
    throw apiError;
  }
};

export const deleteUser: DeleteUserAction = async (id) => {
  try {
    await getAuthUser();
    await serverApi().delete(`/users/${id}`);
  } catch (error) {
    const apiError = error as ApiError;
    console.error("Error deleting user:", apiError);
    throw apiError;
  }
};

export const updateProfileAction: UpdateProfileFormAction = async (
  prevState,
  formData
) => {
  const name = formData.get("name");
  try {
    await serverApi().put<UserResponse>("/user/profile", {
      name,
    });
    return { ...prevState, name };
  } catch (error) {
    const apiError = error as ApiError;
    console.error("Error updating user:", apiError);
    return { ...prevState, name, error: apiError.message };
  }
};
