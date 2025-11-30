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
import { auth as firebaseAuth } from "@/services/firebaseService";
import { handleTokenChanged } from "@/lib/actions/auth.actions";
import { User } from "@/lib/actions/user.types";
import { logout, loginWithTokenAction, registerWithEmailAction, getAuthUser } from "@/lib/actions/auth.actions";
import { LoginWithTokenFormState, RegisterRequest } from "@/lib/actions/auth.types";

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
  initialUser?: User | null;
};

export const AuthProvider = ({ children, initialUser = null }: AuthProviderProps) => {
  const [user, setUser] = useState<User | null>(initialUser);
  const [loading, setLoading] = useState(!initialUser);
  const [, startTransition] = useTransition();
  const [, loginWithToken] = useActionState<undefined, LoginWithTokenFormState>(
    loginWithTokenAction,
    undefined,
  );
  const [, registerWithEmail] = useActionState<undefined, RegisterRequest>(
    registerWithEmailAction,
    undefined,
  );

  /**
   * Handle refresh token changes
   */
  useEffect(() => {
    const unsubscribe = onIdTokenChanged(firebaseAuth, async (firebaseUser) => {
      if (firebaseUser) {
        // Extract ID token on client side (can't pass FirebaseUser to server action - circular refs)
        try {
          const idToken = await firebaseUser.getIdToken();
          await handleTokenChanged(idToken);

          // Fetch user data after token verification
          const user = await getAuthUser({ redirect: false });
          setUser(user);
        } catch (error) {
          console.error("Error getting ID token:", error);
          setUser(null);
        }
      } else {
        setUser(null);
      }
      setLoading(false);
    });

    return () => unsubscribe();
  }, []);

  const signUp = async (email: string, password: string, name: string) => {
    try {
      const userCredential = await createUserWithEmailAndPassword(firebaseAuth, email, password);

      const registerData: RegisterRequest = {
        email,
        name,
        token: userCredential.user.uid,
      };

      startTransition(() => {
        void registerWithEmail(registerData);
      });
    } catch (error: unknown) {
      console.error("Error signing up:", error);
      throw error;
    }
  };

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

  const signIn = async (email: string, password: string) => {
    try {
      const userCredential = await signInWithEmailAndPassword(firebaseAuth, email, password);

      const idToken = await userCredential.user.getIdToken();
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

  const handleLogout = async () => {
    try {
      await signOut(firebaseAuth);
      await logout();
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
        loading,
        signUp,
        signIn,
        signInWithGoogle,
        logout: handleLogout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};
