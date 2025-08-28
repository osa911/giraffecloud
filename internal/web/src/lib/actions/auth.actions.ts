"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import serverApi from "@/services/apiClient/serverApiClient";
import { User as FirebaseUser } from "firebase/auth";
import { UserResponse, User } from "./user.types";
import {
  LoginRequest,
  RegisterRequest,
  VerifyTokenRequest,
  LoginWithTokenFormState,
} from "./auth.types";

const USER_DATA_COOKIE_NAME = "user_data";

export const loginWithTokenAction = async (
  prevState: undefined,
  newState: LoginWithTokenFormState,
): Promise<undefined> => {
  const token = newState.token;
  let user: User | null = null;
  try {
    user = await login({ token });
  } catch (error) {
    console.error("Error logging in:", error);
  } finally {
    if (user) {
      redirect("/dashboard");
    }
  }
};

export async function registerWithEmailAction(
  prevState: undefined,
  newState: RegisterRequest,
): Promise<undefined> {
  let user: User | null = null;
  try {
    user = await register(newState);
  } catch (error) {
    console.error("Error registering:", error);
  } finally {
    if (user) {
      redirect("/dashboard");
    }
  }
}

export async function login(data: LoginRequest): Promise<User> {
  const { user } = await serverApi().post<UserResponse>("/auth/login", data);
  await setUserDataCookie(user);
  return user;
}

export async function register(data: RegisterRequest): Promise<User> {
  const { user } = await serverApi().post<UserResponse>("/auth/register", data);
  await setUserDataCookie(user);
  return user;
}

export async function logout(): Promise<void> {
  await serverApi().post("/auth/logout");
  await setUserDataCookie(null);
  redirect("/auth/login");
}

export async function getAuthUser(): Promise<User>;
export async function getAuthUser(options: { redirect: true }): Promise<User>;
export async function getAuthUser(options: { redirect: false }): Promise<User | null>;
export async function getAuthUser(options = { redirect: true }): Promise<User | null> {
  let user: User | null = null;

  try {
    // Try cookie first
    const cookieData = await getUserDataFromCookie();
    if (cookieData) {
      user = cookieData;
      return user;
    }

    // Fallback to API
    const data = await serverApi().get<{ valid: boolean; user?: User }>("/auth/session");
    if (data.valid && data.user) {
      user = data.user;
      await setUserDataCookie(data.user);
      return user;
    }
  } catch (error) {
    console.error("Error verifying session:", error);
  } finally {
    if (options.redirect && !user) {
      redirect("/auth/login");
    }
  }

  return user;
}

export async function verifyToken(data: VerifyTokenRequest): Promise<void> {
  await serverApi().post("/auth/verify-token", data);
}

// Helper functions
async function setUserDataCookie(user: User | null): Promise<void> {
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
    sameSite: "strict",
  });
}

export async function getUserDataFromCookie(): Promise<User | null> {
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
export async function refreshSessionIfNeeded(user: FirebaseUser): Promise<boolean> {
  try {
    const idToken = await user.getIdToken();
    await verifyToken({ id_token: idToken });
    return true;
  } catch (error) {
    console.error("Error refreshing session cookie:", error);
    return false;
  }
}

export async function handleTokenChanged(user: FirebaseUser): Promise<void> {
  try {
    await refreshSessionIfNeeded(user);
  } catch (error) {
    console.error("Error handling token change:", error);
  }
}
