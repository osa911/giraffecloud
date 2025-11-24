import React from "react";
import SettingsPage from "@/components/dashboard/settings/SettingsPage";

// Force dynamic rendering (auth-protected page)
export const dynamic = "force-dynamic";

export default function SettingsServerPage() {
  return <SettingsPage />;
}
