import { Metadata } from "next";
import ProfilePage from "@/components/dashboard/profile/ProfilePage";
import { getAuthUser } from "@/lib/actions/auth.actions";

export const metadata: Metadata = {
  title: "Profile - GiraffeCloud",
  description: "Manage your profile",
};

// Force dynamic rendering (auth-protected page)
export const dynamic = "force-dynamic";

export default async function ProfilePageRoute() {
  const user = await getAuthUser({ redirect: true });

  return (
    <div className="flex-1 space-y-4">
      <ProfilePage initialUser={user} />
    </div>
  );
}
