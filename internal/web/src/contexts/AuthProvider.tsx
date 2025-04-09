"use client";

import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  useActionState,
  useTransition,
} from "react";
import {
  signInWithEmailAndPassword,
  createUserWithEmailAndPassword,
  signOut,
  onIdTokenChanged,
  GoogleAuthProvider,
  signInWithPopup,
  AuthError,
} from "firebase/auth";
import clientApi from "@/services/api/clientApiClient";
import { auth as firebaseAuth } from "@/services/firebaseService";
import { handleTokenChanged } from "@/services/authClientService";
import {
  handleLogout,
  loginWithTokenAction,
  LoginWithTokenFormState,
  RegisterFormState,
  registerWithEmailAction,
} from "@/lib/actions";

export type User = {
  id: number;
  email: string;
  name: string;
  role: "user" | "admin";
  isActive: boolean;
};

type AuthContextType = {
  user: User | null;
  loading: boolean;
  signUp: (email: string, password: string, name: string) => Promise<void>;
  signIn: (email: string, password: string) => Promise<void>;
  signInWithGoogle: () => Promise<void>;
  logout: () => Promise<void>;
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
  const [isPending, startTransition] = useTransition();
  const [loginState, loginWithToken, isLoginLoading] = useActionState<
    undefined,
    LoginWithTokenFormState
  >(loginWithTokenAction, undefined);
  const [registerState, registerWithEmail, isRegisterLoading] = useActionState<
    undefined,
    RegisterFormState
  >(registerWithEmailAction, undefined);
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

  // Helper function to login with token
  const handleLoginWithToken = async (token: string): Promise<void> => {
    try {
      startTransition(() => {
        void loginWithToken({ token });
      });
    } catch (error) {
      console.error("Error during login:", error);
      setUser(null);
      throw error;
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
      const registerData: RegisterFormState = {
        email,
        name,
        firebase_uid: userCredential.user.uid,
      };

      startTransition(() => {
        void registerWithEmail(registerData);
      });
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
        // TODO: migrate to server action
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
    }
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        loading: isLoginLoading || isRegisterLoading,
        signUp,
        signIn,
        signInWithGoogle,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
