"use server";

import { redirect } from "next/navigation";
import serverApi from "./api/serverApiClient";
import type { User } from "@/contexts/AuthProvider";
import { getUserDataFromCookie } from "@/lib/actions";

/**
 * Gets the authenticated user from the server
 * @returns The authenticated user or null if not authenticated
 */
export async function getServerSession(): Promise<User | null> {
  try {
    // Try to get user data from cookie first (fastest path)
    const cookieData = await getUserDataFromCookie();
    if (cookieData) {
      console.log("getServerSession: using cookie data");
      return cookieData;
    }

    console.log("getServerSession: no cookie data, falling back to API call");

    // Make API call to get session status and user data
    const data = await serverApi().get<{ valid: boolean; user?: User }>(
      "/auth/session"
    );

    if (data.valid && data.user) {
      return data.user;
    }

    return null;
  } catch (error) {
    console.error("Error verifying session:", error);
    return null;
  }
}

/**
 * A simpler utility function to use in server components that
 * returns the authenticated user or redirects
 */
export async function requireAuth(): Promise<User> {
  const user = await getServerSession();

  if (!user) {
    redirect("/login");
  }

  return user;
}
