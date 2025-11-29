import Link from "next/link";
import { Metadata } from "next";

export const metadata: Metadata = {
  title: "About | GiraffeCloud",
  description:
    "Learn about GiraffeCloud and our mission to provide secure, reliable tunneling and reverse proxy services.",
};

export default function AboutPage() {
  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <h1 className="text-4xl font-bold mb-6">About GiraffeCloud</h1>
      <p className="text-lg text-muted-foreground mb-8">
        GiraffeCloud is a secure tunneling and reverse proxy platform designed to help developers
        and businesses expose services safely, reliably, and at scale. We focus on performance,
        security, and operational simplicity.
      </p>

      <h2 className="text-2xl font-semibold mb-4">What we do</h2>
      <ul className="list-disc pl-6 space-y-2 mb-8 text-muted-foreground">
        <li>
          Secure tunnels for HTTP and WebSocket traffic with robust authentication and access controls.
        </li>
        <li>
          Automatic certificate management and modern TLS.
        </li>
        <li>
          Reliability features: health checks, graceful deploys, and observability.
        </li>
      </ul>

      <h2 className="text-2xl font-semibold mb-4">Get in touch</h2>
      <p className="text-muted-foreground">
        Questions or feedback? Visit our <Link href="/contact" className="text-primary hover:underline">Contact</Link> page.
      </p>
    </div>
  );
}
