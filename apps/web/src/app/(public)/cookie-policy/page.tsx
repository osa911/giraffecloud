import { Metadata } from "next";

export const metadata: Metadata = {
  title: "Cookie Policy | GiraffeCloud",
  description: "How GiraffeCloud uses cookies and similar technologies.",
};

export default function CookiePolicyPage() {
  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <h1 className="text-4xl font-bold mb-6">Cookie Policy</h1>
      <p className="text-muted-foreground mb-8">
        We use cookies and similar technologies to provide core functionality (e.g.,
        authentication), remember preferences, and measure product usage.
      </p>

      <div className="space-y-8">
        <section>
          <h2 className="text-2xl font-semibold mb-4">Types of Cookies</h2>
          <ul className="list-disc pl-6 space-y-2 text-muted-foreground">
            <li>
              Strictly necessary: required for login and security.
            </li>
            <li>Preferences: remember settings such as theme.</li>
            <li>
              Analytics: help us understand usage and improve the Service.
            </li>
          </ul>
        </section>

        <p className="text-muted-foreground">
          You can control cookies through your browser settings. Disabling certain cookies may
          impact functionality.
        </p>

        <p className="text-sm text-muted-foreground mt-12">
          Last updated: {new Date().toISOString().split("T")[0]}
        </p>
      </div>
    </div>
  );
}
