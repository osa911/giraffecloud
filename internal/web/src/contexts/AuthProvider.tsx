"use client";

import React, { createContext, useContext, useState, useEffect } from "react";
import {
  getAuth,
  signInWithEmailAndPassword,
  createUserWithEmailAndPassword,
  signOut,
  onIdTokenChanged,
  GoogleAuthProvider,
  signInWithPopup,
  onAuthStateChanged,
  AuthError,
  User as FirebaseUser,
} from "firebase/auth";
import { useRouter } from "next/navigation";
import clientApi from "@/services/api/clientApiClient";
import { auth as firebaseAuth } from "@/services/firebaseService";
import { handleTokenChanged } from "@/services/authClientService";
import { handleLoginSuccess, handleLogout } from "@/lib/actions";

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

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
};

type AuthProviderProps = {
  children: React.ReactNode;
  initialUser: User | null;
};

export function AuthProvider({ children, initialUser }: AuthProviderProps) {
  const [user, setUser] = useState<User | null>(initialUser);
  const [loading, setLoading] = useState(false);

  /**
   * Handle refresh token changes
   */
  useEffect(() => {
    const unsubscribe = onIdTokenChanged(firebaseAuth, async (firebaseUser) => {
      if (firebaseUser) {
        await handleTokenChanged(firebaseUser);
      }
    });

    return () => unsubscribe();
  }, []);

  const updateUser = (updatedUser: User | null) => {
    setUser(updatedUser);
  };

  // Helper function to login with token
  const handleLoginWithToken = async (token: string): Promise<User> => {
    try {
      setLoading(true);
      const loginResponse = await clientApi().post<LoginResponse>(
        "/auth/login",
        {
          token: token,
        }
      );

      // Set user data in cookie via server action
      await handleLoginSuccess(loginResponse.user);

      setUser(loginResponse.user);

      return loginResponse.user;
    } catch (error) {
      console.error("Error during login:", error);
      setUser(null);
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const signUp = async (email: string, password: string, name: string) => {
    try {
      setLoading(true);
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

      const response = await clientApi().post<RegisterResponse>(
        "/auth/register",
        registerData
      );
      // Set user data in cookie via server action
      await handleLoginSuccess(response.user);

      setUser(response.user);
    } catch (error: any) {
      console.error("Error signing up:", error);
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const signIn = async (email: string, password: string) => {
    try {
      setLoading(true);
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
    } finally {
      setLoading(false);
    }
  };

  const signInWithGoogle = async () => {
    const provider = new GoogleAuthProvider();
    try {
      setLoading(true);
      const userCredential = await signInWithPopup(firebaseAuth, provider);
      const idToken = await userCredential.user.getIdToken();

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
    } finally {
      setLoading(false);
    }
  };

  const logout = async () => {
    try {
      setLoading(true);
      // First, try to notify the backend about logout
      try {
        await clientApi().post("/auth/logout");
      } catch (error) {
        console.error("Error notifying backend of logout:", error);
        // Continue with logout even if backend notification fails
      }

      // Clear cookie via server action
      await handleLogout();

      // Then sign out from Firebase
      await signOut(firebaseAuth);
      setUser(null);
    } catch (error) {
      console.error("Error signing out:", error);
      throw error;
    } finally {
      setLoading(false);
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
}
