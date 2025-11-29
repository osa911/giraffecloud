import { Metadata } from "next";
import SettingsPage from "@/components/dashboard/settings/SettingsPage";

export const metadata: Metadata = {
  title: "Settings - GiraffeCloud",
  description: "Manage your account settings",
};

// Force dynamic rendering (auth-protected page)
export const dynamic = "force-dynamic";

export default function SettingsPageRoute() {
  return (
    <div className="flex-1 space-y-4">
      <SettingsPage />
    </div>
  );
}
