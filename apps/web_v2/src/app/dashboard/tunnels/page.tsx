import { Metadata } from "next";
import TunnelList from "@/components/dashboard/tunnels/TunnelList";

export const metadata: Metadata = {
  title: "Tunnels - GiraffeCloud",
  description: "Manage your GiraffeCloud tunnels",
};

// Force dynamic rendering (auth-protected page)
export const dynamic = "force-dynamic";

export default function TunnelsPage() {
  return (
    <div className="flex-1 space-y-4">
      <TunnelList />
    </div>
  );
}
