import { getAuthUser } from "@/lib/actions/auth.actions";
import ProfilePage from "@/components/dashboard/profile/ProfilePage";

export default async function ProfileServerPage() {
  // Get user without updating cache (page components can't modify cookies)
  const user = await getAuthUser({ redirect: true, updateCache: false });
  return <ProfilePage initialUser={user} />;
}
