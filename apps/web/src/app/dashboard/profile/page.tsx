import { getAuthUser } from "@/lib/actions/auth.actions";
import ProfilePage from "@/components/dashboard/profile/ProfilePage";

// Force dynamic rendering (uses cookies for auth)
export const dynamic = "force-dynamic";

export default async function ProfileServerPage() {
  // Get user without updating cache (page components can't modify cookies)
  const user = await getAuthUser({ redirect: true, updateCache: false });
  return <ProfilePage initialUser={user} />;
}
