"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import serverApi from "@/services/apiClient/serverApiClient";
import {
  SESSION_COOKIE_NAME,
  AUTH_TOKEN_COOKIE_NAME,
  USER_DATA_COOKIE_NAME,
  USER_DATA_CACHE_TTL,
  CSRF_COOKIE_NAME,
} from "@/services/apiClient/baseApiClient";
import { ROUTES } from "@/constants/routes";
import { User as FirebaseUser } from "firebase/auth";
import { UserResponse, User } from "./user.types";
import {
  LoginRequest,
  RegisterRequest,
  VerifyTokenRequest,
  LoginWithTokenFormState,
} from "./auth.types";

// Type for cached user data with timestamp
type CachedUserData = {
  user: User;
  cachedAt: number;
};

/**
 * Helper to make auth API calls that set cookies
 * Uses serverApi().postRaw() to access Set-Cookie headers from backend
 */
async function callAuthEndpointWithCookies<T>(
  method: "post",
  endpoint: string,
  data?: unknown,
): Promise<T> {
  // Use serverApi().postRaw() to get full response with headers
  const response = await serverApi().postRaw<T>(endpoint, data);

  // Forward Set-Cookie headers from Go backend to browser
  const setCookieHeaders = response.headers["set-cookie"];
  if (setCookieHeaders) {
    const cookieStore = await cookies();

    setCookieHeaders.forEach((cookieStr) => {
      const parts = cookieStr.split("; ");
      const nameValue = parts[0];
      if (!nameValue) return;

      const nameValueParts = nameValue.split("=");
      const name = nameValueParts[0];
      const value = nameValueParts[1];
      if (!name || !value) return;

      const options = parts.slice(1);

      const cookieOptions: {
        path?: string;
        domain?: string;
        maxAge?: number;
        secure?: boolean;
        httpOnly?: boolean;
        sameSite?: "lax" | "strict" | "none" | boolean;
      } = {};
      options.forEach((opt) => {
        const [key, val = true] = opt.toLowerCase().split("=");
        if (!key) return;

        switch (key) {
          case "path":
            if (typeof val === "string") {
              cookieOptions.path = val;
            }
            break;
          case "domain":
            if (typeof val === "string") {
              cookieOptions.domain = val;
            }
            break;
          case "max-age":
            if (typeof val === "string") {
              cookieOptions.maxAge = parseInt(val);
            }
            break;
          case "secure":
            cookieOptions.secure = true;
            break;
          case "httponly":
            cookieOptions.httpOnly = true;
            break;
          case "samesite":
            // Type guard for sameSite values
            if (typeof val === "string") {
              const lowerVal = val.toLowerCase();
              if (lowerVal === "lax" || lowerVal === "strict" || lowerVal === "none") {
                cookieOptions.sameSite = lowerVal;
              }
            } else {
              cookieOptions.sameSite = val;
            }
            break;
        }
      });

      cookieStore.set(name, value, cookieOptions);
    });
  }

  // Return data in the same format as serverApi()
  return response.data.data as T;
}

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
      redirect(ROUTES.DASHBOARD.HOME);
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
      redirect(ROUTES.DASHBOARD.HOME);
    }
  }
}

export async function login(data: LoginRequest): Promise<User> {
  const { user } = await callAuthEndpointWithCookies<UserResponse>("post", "/auth/login", data);
  await setUserDataCookie(user);
  return user;
}

export async function register(data: RegisterRequest): Promise<User> {
  const { user } = await callAuthEndpointWithCookies<UserResponse>("post", "/auth/register", data);
  await setUserDataCookie(user);
  return user;
}

export async function logout(): Promise<void> {
  await serverApi().post("/auth/logout");
  await clearAllAuthCookies();
  redirect(ROUTES.AUTH.LOGIN);
}

/**
 * Helper to clear all authentication cookies (SERVER-SIDE ONLY)
 *
 * This is the centralized server-side implementation used by:
 * - logout()
 * - getAuthUser() on errors (catches 401/403 from API)
 *
 * NOTE: There's a similar clearAuthCookies() in clientApiClient.ts for client-side.
 * They can't be shared because:
 * - This uses Next.js cookies() API (server-side)
 * - Client uses document.cookie (browser API)
 * - Server actions ("use server") can't be imported in client code
 * - This can ONLY be called from Server Actions, not from axios interceptors
 *
 * IMPORTANT: Don't call this from serverApiClient interceptor - it runs during
 * SSR where cookie modifications aren't allowed by Next.js.
 *
 * If you add/remove cookies, update BOTH implementations!
 */
export async function clearAllAuthCookies(): Promise<void> {
  const cookieStore = await cookies();
  cookieStore.delete(SESSION_COOKIE_NAME);
  cookieStore.delete(AUTH_TOKEN_COOKIE_NAME);
  cookieStore.delete(CSRF_COOKIE_NAME);
  cookieStore.delete(USER_DATA_COOKIE_NAME);
}

export async function getAuthUser(): Promise<User>;
export async function getAuthUser(options: { redirect: true }): Promise<User>;
export async function getAuthUser(options: { redirect: false }): Promise<User | null>;
export async function getAuthUser(options = { redirect: true }): Promise<User | null> {
  let user: User | null = null;

  try {
    // First, try to get user from cache (fast path)
    const cachedUser = await getCachedUser();
    if (cachedUser) {
      // Cache is fresh and valid - return immediately
      return cachedUser;
    }

    // Cache miss or expired - need to validate with backend
    const cookieStore = await cookies();
    const sessionCookie = cookieStore.get(SESSION_COOKIE_NAME);
    const authTokenCookie = cookieStore.get(AUTH_TOKEN_COOKIE_NAME);
    const hasSessionCookies = !!(sessionCookie || authTokenCookie);

    if (!hasSessionCookies) {
      // No session cookies - user is not authenticated
      return null;
    }

    // We have session cookies but no fresh cache - validate with API
    try {
      const data = await serverApi().get<{ valid: boolean; user?: User }>("/auth/session");
      if (data.valid && data.user) {
        user = data.user;
        await setUserDataCookie(data.user);
        return user;
      } else {
        // Session is invalid - clear everything
        await clearAllAuthCookies();
        return null;
      }
    } catch {
      // API call failed (likely 401) - clear stale cookies
      await clearAllAuthCookies();
      return null;
    }
  } catch (error) {
    console.error("Error verifying session:", error);
    // API call failed (likely 401/403) - clear all auth cookies
    // This may fail if called during SSR, which is okay
    try {
      await clearAllAuthCookies();
    } catch (cookieError) {
      // Cookie clearing failed - likely during SSR, safe to ignore
      if (process.env.NODE_ENV === "development") {
        const errorMessage =
          cookieError instanceof Error ? cookieError.message : String(cookieError);
        if (!errorMessage.includes("Server Action or Route Handler")) {
          console.warn("Failed to clear auth cookies:", errorMessage);
        }
      }
    }
  } finally {
    if (options.redirect && !user) {
      redirect(ROUTES.AUTH.LOGIN);
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

  const cachedData: CachedUserData = {
    user,
    cachedAt: Date.now(),
  };

  cookieStore.set(USER_DATA_COOKIE_NAME, JSON.stringify(cachedData), {
    httpOnly: true,
    path: "/",
    secure: process.env.NODE_ENV === "production",
    maxAge: 60 * 60 * 24 * 1, // 1 day
    sameSite: "strict",
  });
}

async function getCachedUserData(): Promise<CachedUserData | null> {
  try {
    const cookieStore = await cookies();
    const userDataCookie = cookieStore.get(USER_DATA_COOKIE_NAME);
    if (userDataCookie?.value) {
      const parsed = JSON.parse(userDataCookie.value);

      // Support both old format (just User) and new format (CachedUserData)
      if (parsed.user && parsed.cachedAt) {
        return parsed as CachedUserData;
      } else {
        // Old format - migrate by adding timestamp
        return {
          user: parsed as User,
          cachedAt: Date.now(),
        };
      }
    }
    return null;
  } catch (error) {
    console.error("Error getting cached user data:", error);
    return null;
  }
}

export async function getUserDataFromCookie(): Promise<User | null> {
  const cachedData = await getCachedUserData();
  return cachedData?.user || null;
}

/**
 * Gets cached user data without making API calls or redirecting
 *
 * Use this when:
 * - You need user data for UI purposes (display name, email, etc.)
 * - You don't need to validate the session
 * - You're okay with potentially stale data (up to 5 minutes old)
 *
 * Don't use this when:
 * - You need to verify authentication (use getAuthUser instead)
 * - You need fresh data from the backend
 * - You're in a security-critical component
 *
 * @returns User object from cache or null if not found/expired
 */
export async function getCachedUser(): Promise<User | null> {
  try {
    const cachedData = await getCachedUserData();
    if (!cachedData) return null;

    const { user, cachedAt } = cachedData;
    const now = Date.now();
    const cacheAge = now - cachedAt;

    // Return user only if cache is still fresh
    if (cacheAge < USER_DATA_CACHE_TTL) {
      return user;
    }

    // Cache expired - return null
    return null;
  } catch (error) {
    console.error("Error getting cached user:", error);
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
