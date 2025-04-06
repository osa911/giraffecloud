"use server";

import { cookies } from "next/headers";
import { User } from "@/contexts/AuthProvider";

const USER_DATA_COOKIE_NAME = "user_data";

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
