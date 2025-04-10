import React from "react";
import type { Metadata } from "next";
import { Inter } from "next/font/google";
import { AppRouterCacheProvider } from "@mui/material-nextjs/v15-appRouter";
import { AuthProvider } from "@/contexts/AuthProvider";
import { ThemeProvider } from "@mui/material/styles";
import CssBaseline from "@mui/material/CssBaseline";
import theme from "@/styles/theme";
import { Toaster } from "react-hot-toast";
import { getServerSession } from "@/services/authServerService";
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

type RootLayoutProps = {
  children: React.ReactNode;
};

export default async function RootLayout({ children }: RootLayoutProps) {
  const initialUser = await getServerSession();

  return (
    <html lang="en" className={inter.className}>
      <body>
        <AppRouterCacheProvider>
          <ThemeProvider theme={theme}>
            <CssBaseline />
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
