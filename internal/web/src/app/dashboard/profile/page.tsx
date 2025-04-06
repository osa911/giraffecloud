import { requireAuth } from "@/services/authServerService";
import ProfileForm from "@/components/dashboard/profile/ProfileForm";

export default async function ProfilePage() {
  // Get authenticated user (already checked by dashboard layout, but good to be explicit)
  const user = await requireAuth();

  return <ProfileForm initialUser={user} />;
}
