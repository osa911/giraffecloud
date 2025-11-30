import { Metadata } from "next";

export const metadata: Metadata = {
  title: "Terms of Service | GiraffeCloud",
  description: "The terms that govern use of GiraffeCloud services.",
};

export default function TermsPage() {
  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <h1 className="text-4xl font-bold mb-6">Terms of Service</h1>
      <p className="text-muted-foreground mb-8">
        These Terms of Service (&quot;Terms&quot;) govern your access to and use of the
        GiraffeCloud services (the &quot;Service&quot;). By using the Service, you agree to be
        bound by these Terms.
      </p>

      <div className="space-y-8">
        <section>
          <h2 className="text-2xl font-semibold mb-4">1. Accounts and Eligibility</h2>
          <p className="text-muted-foreground">
            You must be at least 18 years old and have the legal capacity to enter into these Terms.
            You are responsible for safeguarding your account credentials.
          </p>
        </section>

        <section>
          <h2 className="text-2xl font-semibold mb-4">2. Acceptable Use</h2>
          <p className="text-muted-foreground">
            You may not use the Service to violate any law, infringe intellectual property rights,
            transmit malware, perform denial-of-service attacks, or abuse network resources. We may
            suspend or terminate accounts engaged in prohibited activity.
          </p>
        </section>

        <section>
          <h2 className="text-2xl font-semibold mb-4">3. Fees and Billing</h2>
          <p className="text-muted-foreground">
            Paid plans are billed in advance on a subscription basis. Charges are non-refundable
            except as required by law or as described in our Refund Policy.
          </p>
        </section>

        <section>
          <h2 className="text-2xl font-semibold mb-4">4. Service Availability</h2>
          <p className="text-muted-foreground">
            We strive for high availability but do not guarantee uninterrupted service. We may perform
            maintenance or updates that temporarily affect the Service.
          </p>
        </section>

        <section>
          <h2 className="text-2xl font-semibold mb-4">5. Data and Privacy</h2>
          <p className="text-muted-foreground">
            Our Privacy Policy explains how we collect and process personal data. You consent to our
            data practices by using the Service.
          </p>
        </section>

        <section>
          <h2 className="text-2xl font-semibold mb-4">6. Disclaimers and Limitation of Liability</h2>
          <p className="text-muted-foreground">
            The Service is provided &quot;as is&quot; without warranties of any kind. To the maximum
            extent permitted by law, GiraffeCloud shall not be liable for indirect or consequential
            damages.
          </p>
        </section>

        <section>
          <h2 className="text-2xl font-semibold mb-4">7. Termination</h2>
          <p className="text-muted-foreground">
            We may suspend or terminate your access if you breach these Terms. You may stop using the
            Service at any time.
          </p>
        </section>

        <section>
          <h2 className="text-2xl font-semibold mb-4">8. Changes</h2>
          <p className="text-muted-foreground">
            We may update these Terms from time to time. If changes are material, we will provide
            notice by posting an update on this page or via email.
          </p>
        </section>

        <p className="text-sm text-muted-foreground mt-12">
          Last updated: {new Date().toISOString().split("T")[0]}
        </p>
      </div>
    </div>
  );
}
