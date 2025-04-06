import { User as FirebaseUser, getIdToken } from "firebase/auth";
import { apiClient } from "@/services/api";

/**
 * Refreshes the session cookie with the backend when needed
 * To be called after Firebase automatically refreshes the ID token
 * @param user Current Firebase user
 * @returns Promise that resolves to true if successful
 */
export async function refreshSessionIfNeeded(
  user: FirebaseUser
): Promise<boolean> {
  try {
    // Get the current ID token from Firebase (will be fresh due to Firebase's auto-refresh)
    const idToken = await user.getIdToken();

    // Call the backend to refresh the session cookie
    await apiClient().post("/auth/verify-token", {
      id_token: idToken,
    });

    console.log("Session cookie refreshed with backend");
    return true;
  } catch (error) {
    console.error("Error refreshing session cookie:", error);
    return false;
  }
}

/**
 * Ensures that when claims change, the session is refreshed
 * @param user Firebase user object
 */
export async function handleTokenChanged(user: FirebaseUser): Promise<void> {
  try {
    // This will be called when the token is refreshed by Firebase
    console.log("Token refreshed by Firebase, updating session cookie");
    await refreshSessionIfNeeded(user);
  } catch (error) {
    console.error("Error handling token change:", error);
  }
}
