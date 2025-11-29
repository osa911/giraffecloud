import { Metadata } from "next";

export const metadata: Metadata = {
  title: "Refund Policy | GiraffeCloud",
  description: "Refund terms for GiraffeCloud paid plans.",
};

export default function RefundPolicyPage() {
  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <h1 className="text-4xl font-bold mb-6">Refund Policy</h1>
      <p className="text-muted-foreground mb-8">
        Subscriptions are billed in advance and are non-refundable, except where required by law
        or if we fail to deliver the core service for a prolonged period not caused by you or
        third-party dependencies.
      </p>

      <div className="space-y-8">
        <section>
          <h2 className="text-2xl font-semibold mb-4">Cancellations</h2>
          <p className="text-muted-foreground">
            You can cancel any time. Your subscription remains active until the end of the current
            billing period.
          </p>
        </section>

        <section>
          <h2 className="text-2xl font-semibold mb-4">Billing Disputes</h2>
          <p className="text-muted-foreground">
            If you believe there is an error, contact support within 30 days of the charge. We will
            review and, if appropriate, issue a credit.
          </p>
        </section>

        <p className="text-sm text-muted-foreground mt-12">
          Last updated: {new Date().toISOString().split("T")[0]}
        </p>
      </div>
    </div>
  );
}
