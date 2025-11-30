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

      <h2 className="text-2xl font-semibold mb-4">Our Mission</h2>
      <p className="text-muted-foreground mb-8">
        To simplify the way developers share and expose their local work to the world, without compromising on security or speed.
        We believe that setting up secure remote access should be as easy as typing a single command.
      </p>

      <h2 className="text-2xl font-semibold mb-4">Why GiraffeCloud?</h2>
      <ul className="list-disc pl-6 space-y-3 mb-8 text-muted-foreground">
        <li>
          <strong className="text-foreground">Instant Sharing:</strong> Turn your localhost server into a public URL in seconds. Perfect for client demos, webhook testing, and mobile debugging.
        </li>
        <li>
          <strong className="text-foreground">Secure by Design:</strong> We handle the complexity of SSL/TLS certificates automatically. Your tunnels are encrypted and protected with robust access controls.
        </li>
        <li>
          <strong className="text-foreground">Developer First:</strong> Built for modern workflows with a powerful CLI, intuitive dashboard, and reliable connections that don't drop when you need them most.
        </li>
      </ul>

      <h2 className="text-2xl font-semibold mb-4">Get in touch</h2>
      <p className="text-muted-foreground">
        Questions or feedback? Visit our <Link href="/contact" className="text-primary hover:underline">Contact</Link> page.
      </p>
    </div>
  );
}
