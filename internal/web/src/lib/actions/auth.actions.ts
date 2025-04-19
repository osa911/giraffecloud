"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import serverApi from "@/services/apiClient/serverApiClient";
import { User as FirebaseUser } from "firebase/auth";
import { UserResponse } from "./user.types";
import {
  LoginRequest,
  RegisterRequest,
  VerifyTokenRequest,
  LoginWithTokenFormState,
} from "./auth.types";

const USER_DATA_COOKIE_NAME = "user_data";

export const loginWithTokenAction = async (
  prevState: undefined,
  newState: LoginWithTokenFormState
): Promise<undefined> => {
  const token = newState.token;
  try {
    const user = await login({ token });
    redirect("/dashboard");
  } catch (error) {
    console.error("Error logging in:", error);
  }
};

export async function registerWithEmailAction(
  prevState: undefined,
  newState: RegisterRequest
): Promise<undefined> {
  try {
    const user = await register(newState);
    redirect("/dashboard");
  } catch (error) {
    console.error("Error registering:", error);
  }
}

export async function login(data: LoginRequest): Promise<UserResponse> {
  const user = await serverApi().post<UserResponse>("/auth/login", data);
  await setUserDataCookie(user);
  return user;
}

export async function register(data: RegisterRequest): Promise<UserResponse> {
  const user = await serverApi().post<UserResponse>("/auth/register", data);
  await setUserDataCookie(user);
  return user;
}

export async function logout(): Promise<void> {
  await serverApi().post("/auth/logout");
  await setUserDataCookie(null);
  redirect("/auth/login");
}

export async function getAuthUser(): Promise<UserResponse>;
export async function getAuthUser(options: {
  redirect: true;
}): Promise<UserResponse>;
export async function getAuthUser(options: {
  redirect: false;
}): Promise<UserResponse | null>;
export async function getAuthUser(
  options = { redirect: true }
): Promise<UserResponse | null> {
  let user: UserResponse | null = null;

  try {
    // Try cookie first
    const cookieData = await getUserDataFromCookie();
    if (cookieData) {
      return cookieData;
    }

    // Fallback to API
    const data = await serverApi().get<{ valid: boolean; user?: UserResponse }>(
      "/auth/session"
    );
    if (data.valid && data.user) {
      await setUserDataCookie(data.user);
      return data.user;
    }
  } catch (error) {
    console.error("Error verifying session:", error);
  } finally {
    if (options.redirect && !user) {
      redirect("/auth/login");
    }
  }

  return null;
}

export async function verifyToken(data: VerifyTokenRequest): Promise<void> {
  await serverApi().post("/auth/verify-token", data);
}

// Helper functions
async function setUserDataCookie(user: UserResponse | null): Promise<void> {
  const cookieStore = await cookies();
  if (!user) {
    cookieStore.delete(USER_DATA_COOKIE_NAME);
    return;
  }

  cookieStore.set(USER_DATA_COOKIE_NAME, JSON.stringify(user), {
    httpOnly: true,
    path: "/",
    secure: process.env.NODE_ENV === "production",
    maxAge: 60 * 60 * 24 * 1, // 1 day
  });
}

export async function getUserDataFromCookie(): Promise<UserResponse | null> {
  try {
    const cookieStore = await cookies();
    const userDataCookie = cookieStore.get(USER_DATA_COOKIE_NAME);
    if (userDataCookie?.value) {
      return JSON.parse(userDataCookie.value);
    }
    return null;
  } catch (error) {
    console.error("Error getting user data from cookie:", error);
    return null;
  }
}

// Firebase token handling
export async function refreshSessionIfNeeded(
  user: FirebaseUser
): Promise<boolean> {
  try {
    // Get the current ID token from Firebase (will be fresh due to Firebase's auto-refresh)
    const idToken = await user.getIdToken();

    // Call the backend to refresh the session cookie
    await verifyToken({ id_token: idToken });

    console.log("Session cookie refreshed with backend");
    return true;
  } catch (error) {
    console.error("Error refreshing session cookie:", error);
    return false;
  }
}

export async function handleTokenChanged(user: FirebaseUser): Promise<void> {
  try {
    // This will be called when the token is refreshed by Firebase
    console.log("Token refreshed by Firebase, updating session cookie");
    await refreshSessionIfNeeded(user);
  } catch (error) {
    console.error("Error handling token change:", error);
  }
}
