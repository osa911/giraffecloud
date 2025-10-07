import React from "react";
import type { Metadata } from "next";
import { Inter } from "next/font/google";
import { AppRouterCacheProvider } from "@mui/material-nextjs/v15-appRouter";
import { AuthProvider } from "@/contexts/AuthProvider";
import CookieValidator from "@/components/auth/CookieValidator";
import { ThemeProvider } from "@mui/material/styles";
import CssBaseline from "@mui/material/CssBaseline";
import theme from "@/styles/theme";
import { Toaster } from "react-hot-toast";
import { getAuthUser } from "@/lib/actions/auth.actions";
import { Analytics } from "@vercel/analytics/next";
import { SpeedInsights } from "@vercel/speed-insights/next";

const inter = Inter({
  subsets: ["latin"],
  display: "swap",
  variable: "--font-inter",
  weight: ["300", "400", "500", "700"],
});

export const metadata: Metadata = {
  title: "GiraffeCloud",
  description: "Secure tunnel service for your applications",
};

type RootServerLayoutProps = {
  children: React.ReactNode;
};

export default async function RootServerLayout({ children }: RootServerLayoutProps) {
  const initialUser = await getAuthUser({ redirect: false });

  return (
    <html lang="en" className={inter.className}>
      <body>
        <AppRouterCacheProvider>
          <ThemeProvider theme={theme}>
            <CssBaseline />
            <CookieValidator hasServerAuth={!!initialUser} />
            <AuthProvider initialUser={initialUser}>{children}</AuthProvider>
            <Toaster />
          </ThemeProvider>
        </AppRouterCacheProvider>
        <Analytics />
        <SpeedInsights />
      </body>
    </html>
  );
}
