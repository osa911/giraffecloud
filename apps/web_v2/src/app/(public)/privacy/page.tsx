import { Metadata } from "next";

export const metadata: Metadata = {
  title: "Privacy Policy | GiraffeCloud",
  description: "How GiraffeCloud collects, uses, and protects your data.",
};

export default function PrivacyPolicyPage() {
  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <h1 className="text-4xl font-bold mb-6">Privacy Policy</h1>
      <p className="text-muted-foreground mb-8">
        This Privacy Policy describes how GiraffeCloud (&quot;we&quot;, &quot;us&quot;) collects
        and uses personal information in connection with our services.
      </p>

      <div className="space-y-8">
        <section>
          <h2 className="text-2xl font-semibold mb-4">Information We Collect</h2>
          <ul className="list-disc pl-6 space-y-2 text-muted-foreground">
            <li>
              Account data: name, email, authentication identifiers.
            </li>
            <li>
              Service metadata: IP addresses, device and browser information, usage analytics.
            </li>
            <li>
              Billing data for paid plans processed via our payment provider.
            </li>
          </ul>
        </section>

        <section>
          <h2 className="text-2xl font-semibold mb-4">How We Use Information</h2>
          <ul className="list-disc pl-6 space-y-2 text-muted-foreground">
            <li>Provide and improve the Service</li>
            <li>
              Secure our platform, prevent abuse, and troubleshoot issues
            </li>
            <li>Process payments and manage subscriptions</li>
          </ul>
        </section>

        <section>
          <h2 className="text-2xl font-semibold mb-4">Data Sharing</h2>
          <p className="text-muted-foreground">
            We share data with vendors who help us operate the Service (e.g., cloud hosting,
            analytics, payments). We do not sell personal information.
          </p>
        </section>

        <section>
          <h2 className="text-2xl font-semibold mb-4">Your Choices</h2>
          <p className="text-muted-foreground">
            You may access, correct, or delete certain personal information. You may opt out of
            marketing communications. Some functional communications are required for service
            delivery.
          </p>
        </section>

        <p className="text-sm text-muted-foreground mt-12">
          Last updated: {new Date().toISOString().split("T")[0]}
        </p>
      </div>
    </div>
  );
}
