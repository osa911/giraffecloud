import { Metadata } from "next";
import TunnelList from "@/components/dashboard/tunnels/TunnelList";

export const metadata: Metadata = {
  title: "Tunnels - GiraffeCloud",
  description: "Manage your GiraffeCloud tunnels",
};

export default function TunnelsPage() {
  return <TunnelList />;
}
