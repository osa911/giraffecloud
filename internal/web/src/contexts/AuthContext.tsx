"use client";

import React, { createContext, useContext, useState, useEffect } from "react";
import {
  signInWithEmailAndPassword,
  createUserWithEmailAndPassword,
  signOut,
  onIdTokenChanged,
  GoogleAuthProvider,
  signInWithPopup,
  AuthError,
} from "firebase/auth";
import { apiClient } from "@/services/api";
import { auth as firebaseAuth } from "@/services/firebase";
import { usePathname, useRouter } from "next/navigation";
import { handleTokenChanged } from "@/services/auth-utils";

export type User = {
  id: number;
  email: string;
  name: string;
  role: "user" | "admin";
  isActive: boolean;
};

type LoginResponse = {
  user: User;
};

type RegisterResponse = {
  user: User;
};

type AuthContextType = {
  user: User | null;
  loading: boolean;
  signUp: (email: string, password: string, name: string) => Promise<void>;
  signIn: (email: string, password: string) => Promise<void>;
  signInWithGoogle: () => Promise<void>;
  logout: () => Promise<void>;
  updateUser: (user: User) => void;
};

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const pathname = usePathname();
  const router = useRouter();

  const updateUser = (user: User | null) => {
    setUser(user);
  };

  // Check session status when component mounts
  useEffect(() => {
    const checkSession = async () => {
      setLoading(true);
      try {
        console.log("Checking session validity");
        const sessionData = await apiClient().get<{
          valid: boolean;
          user?: User;
        }>("/auth/session");

        console.log("Session result", sessionData);
        if (sessionData.valid && sessionData.user) {
          console.log("Session is valid, setting user");
          updateUser(sessionData.user);
        } else {
          console.log("No valid session found");
          updateUser(null);

          // If not on auth pages, redirect to login
          if (!["/auth/login", "/auth/register", "/"].includes(pathname)) {
            router.push("/auth/login");
          }
        }
      } catch (error) {
        console.error("Error checking session:", error);
        updateUser(null);
      } finally {
        setLoading(false);
      }
    };

    void checkSession();
  }, [pathname]);

  // Set up session refresh when authenticated
  useEffect(() => {
    if (!user) return;

    // Listen for ID token changes and refresh the session cookie when needed
    const unsubscribe = onIdTokenChanged(firebaseAuth, async (firebaseUser) => {
      if (firebaseUser) {
        // When the ID token changes, refresh the session cookie
        await handleTokenChanged(firebaseUser);
      }
    });

    // Clean up listener when component unmounts
    return () => unsubscribe();
  }, [user]);

  /**
   * If the user is authenticated and on the login or register page, redirect to the dashboard
   */
  useEffect(() => {
    if (user && ["/auth/login", "/auth/register"].includes(pathname)) {
      router.push("/dashboard");
    }
  }, [user, pathname, router]);

  // Helper function to login with token
  const handleLoginWithToken = async (token: string) => {
    try {
      console.log("Logging in with token");

      // The response contains a nested user object
      const loginResponse = await apiClient().post<LoginResponse>(
        "/auth/login",
        {
          token: token,
        }
      );
      console.log("Login successful", loginResponse);
      updateUser(loginResponse.user);
    } catch (error) {
      console.error("Error during login:", error);
      updateUser(null);
    }
  };

  const signUp = async (email: string, password: string, name: string) => {
    try {
      // Create user in Firebase
      const userCredential = await createUserWithEmailAndPassword(
        firebaseAuth,
        email,
        password
      );

      // Create user in backend
      const registerData = {
        email,
        name,
        firebase_uid: userCredential.user.uid,
      };

      console.log("Sending registration request:", registerData);
      const response = await apiClient().post<RegisterResponse>(
        "/auth/register",
        registerData
      );

      console.log("Registration successful");
      updateUser(response.user);
    } catch (error: any) {
      console.error("Error signing up:", error);
      throw error;
    }
  };

  const signIn = async (email: string, password: string) => {
    try {
      const userCredential = await signInWithEmailAndPassword(
        firebaseAuth,
        email,
        password
      );

      const idToken = await userCredential.user.getIdToken();

      // Login to sync with backend
      await handleLoginWithToken(idToken);
    } catch (error) {
      console.error("Error signing in:", error);
      throw error;
    }
  };

  const signInWithGoogle = async () => {
    const provider = new GoogleAuthProvider();
    try {
      const userCredential = await signInWithPopup(firebaseAuth, provider);
      const idToken = await userCredential.user.getIdToken();
      console.log("Google sign-in successful");

      // Login to sync with backend
      await handleLoginWithToken(idToken);
    } catch (error) {
      const authError = error as AuthError;
      if (authError.code === "auth/popup-closed-by-user") {
        provider.setCustomParameters({
          prompt: "select_account",
          login_hint: "",
        });
        throw new Error("popup-closed");
      }
      console.error("Error signing in with Google:", error);
      throw error;
    }
  };

  const logout = async () => {
    try {
      // First, try to notify the backend about logout
      try {
        await apiClient().post("/auth/logout");
        console.log("Backend notified of logout");
      } catch (error) {
        console.error("Error notifying backend of logout:", error);
        // Continue with logout even if backend notification fails
      }

      // Then sign out from Firebase
      await signOut(firebaseAuth);
      updateUser(null);
      router.push("/auth/login");
    } catch (error) {
      console.error("Error signing out:", error);
      throw error;
    }
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        loading,
        signUp,
        signIn,
        signInWithGoogle,
        logout,
        updateUser,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
};
