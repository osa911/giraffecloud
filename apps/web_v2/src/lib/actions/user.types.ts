// Response types
export type User = {
  id: number;
  email: string;
  name: string;
  role: string;
  createdAt: string;
  updatedAt: string;
  picture?: string;
};

export type UserResponse = {
  user: User;
};

export type ListUsersResponse = {
  users: UserResponse[];
  total: number;
  page: number;
  pageSize: number;
};

// Request types
export type UpdateProfileRequest = {
  name?: string;
};

export type UpdateUserRequest = {
  name?: string;
  role?: string;
};

// Form state types
export type FormState = {
  name: string;
  email: string;
  error?: string;
};

// Action types
export type GetProfileAction = () => Promise<UserResponse>;
export type UpdateProfileAction = (data: UpdateProfileRequest) => Promise<UserResponse>;
export type DeleteProfileAction = () => Promise<void>;

export type ListUsersAction = (page?: number, pageSize?: number) => Promise<ListUsersResponse>;
export type GetUserAction = (id: number) => Promise<UserResponse>;
export type UpdateUserAction = (id: number, data: UpdateUserRequest) => Promise<UserResponse>;
export type DeleteUserAction = (id: number) => Promise<void>;

export type UpdateProfileFormAction = (
  prevState: FormState,
  formData: FormData,
) => Promise<FormState>;

// Error types
export type ApiError = {
  message: string;
  code?: string;
  status?: number;
};
