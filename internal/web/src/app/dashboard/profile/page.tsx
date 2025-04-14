import { getAuthUser } from "@/lib/actions/auth.actions";
import ProfilePage from "@/components/dashboard/profile/ProfilePage";

export default async function ProfileServerPage() {
  const user = await getAuthUser();
  return <ProfilePage initialUser={user} />;
}
