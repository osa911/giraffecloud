import { Metadata } from "next";
import GettingStartedPage from "@/components/dashboard/GettingStartedPage";

export const metadata: Metadata = {
  title: "Getting Started - GiraffeCloud",
  description: "Get started with GiraffeCloud",
};

// Force dynamic rendering (auth-protected page)
export const dynamic = "force-dynamic";

export default function GettingStartedPageRoute() {
  return (
    <div className="flex-1 space-y-4">
      <GettingStartedPage />
    </div>
  );
}
