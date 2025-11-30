import { Metadata } from "next";

export const metadata: Metadata = {
  title: "Acceptable Use Policy | GiraffeCloud",
  description: "Rules for acceptable use of GiraffeCloud services.",
};

export default function AcceptableUsePage() {
  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <h1 className="text-4xl font-bold mb-6">Acceptable Use Policy</h1>
      <p className="text-muted-foreground mb-8">
        To keep our network safe and reliable for everyone, you agree not to misuse the Service.
        Prohibited uses include:
      </p>

      <ul className="list-disc pl-6 space-y-4 mb-8 text-muted-foreground">
        <li>
          Illegal content or activity, including infringement and distribution of malware.
        </li>
        <li>
          Denial-of-service attacks, port scanning at scale, or evasion of abuse detection.
        </li>
        <li>
          Sending spam or abusive traffic; crypto mining; child sexual abuse material (CSAM).
        </li>
      </ul>

      <p className="text-muted-foreground">
        We reserve the right to suspend or terminate accounts that violate this policy.
      </p>

      <p className="text-sm text-muted-foreground mt-12">
        Last updated: 2025-11-30
      </p>
    </div>
  );
}
