// Request types
export type LoginRequest = {
  token: string;
};

export type RegisterRequest = {
  token: string;
  email: string;
  name: string;
};

export type VerifyTokenRequest = {
  id_token: string;
};

// Form state types
export type LoginWithTokenFormState = {
  token: string;
};
