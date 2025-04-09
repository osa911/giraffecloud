"use server";

import { cookies } from "next/headers";
import { User } from "@/contexts/AuthProvider";
import serverApi from "@/services/api/serverApiClient";
import { redirect } from "next/navigation";

const USER_DATA_COOKIE_NAME = "user_data";

export type FormState = {
  name: FormDataEntryValue | null;
};

export async function updateProfileAction(
  prevState: FormState,
  formData: FormData
) {
  const name = formData.get("name");
  try {
    await serverApi().put<User>("/user/profile", {
      name,
    });
    return { ...prevState, name };
  } catch (error) {
    console.error("Error updating user:", error);
    return { ...prevState, name };
  }
}

export type LoginWithTokenFormState = {
  token: string;
};

export const loginWithTokenAction = async (
  prevState: undefined,
  newState: LoginWithTokenFormState
): Promise<undefined> => {
  const token = newState.token;
  let shouldRedirect = false;
  try {
    const { user } = await serverApi().post<{ user: User }>("/auth/login", {
      token,
    });
    shouldRedirect = await handleLoginSuccess(user);
  } catch (error) {
    console.error("Error logging in:", error);
  } finally {
    if (shouldRedirect) {
      redirect("/dashboard");
    }
  }
};

export type RegisterFormState = {
  email: string;
  name: string;
  firebase_uid: string;
};

export const registerWithEmailAction = async (
  prevState: undefined,
  newState: RegisterFormState
): Promise<undefined> => {
  let shouldRedirect = false;
  try {
    const { user } = await serverApi().post<{
      user: User;
    }>("/auth/register", newState);
    shouldRedirect = await handleLoginSuccess(user);
  } catch (error) {
    console.error("Error registering:", error);
  } finally {
    if (shouldRedirect) {
      redirect("/dashboard");
    }
  }
};

/**
 * Server action to set user data in cookie
 * Call this directly from client components after successful authentication
 */
export async function handleLoginSuccess(user: User | null) {
  "use server";

  if (!user) return false;

  try {
    const cookieStore = await cookies();

    cookieStore.set({
      name: USER_DATA_COOKIE_NAME,
      value: JSON.stringify(user),
      httpOnly: true,
      path: "/",
      secure: process.env.NODE_ENV === "production",
      maxAge: 60 * 60 * 24 * 1, // 1 day
    });

    return true;
  } catch (error) {
    console.error("Error handling login success:", error);
    return false;
  }
}

/**
 * Server action to clear user data cookie
 * Call this directly from client components during logout
 */
export async function handleLogout() {
  "use server";

  try {
    const cookieStore = await cookies();
    cookieStore.delete(USER_DATA_COOKIE_NAME);
    return true;
  } catch (error) {
    console.error("Error handling logout:", error);
    return false;
  }
}

/**
 * Gets the user data from the cookie
 * This is called server-side only
 */
export async function getUserDataFromCookie(): Promise<User | null> {
  "use server";
  try {
    const cookieStore = await cookies();
    const userDataCookie = cookieStore.get(USER_DATA_COOKIE_NAME);
    if (userDataCookie?.value) {
      try {
        return JSON.parse(userDataCookie.value);
      } catch (error) {
        console.error("Error parsing user data from cookie:", error);
        return null;
      }
    }
    return null;
  } catch (error) {
    console.error("Error getting user data from cookie:", error);
    return null;
  }
}
