import { Metadata } from "next";
import ContactForm from "@/components/common/ContactForm";

export const metadata: Metadata = {
  title: "Contact | GiraffeCloud",
  description: "Contact GiraffeCloud support and sales.",
};

export default function ContactPage() {
  return (
    <div className="container mx-auto px-4 py-12 max-w-xl">
      <h1 className="text-4xl font-bold mb-6 text-center">Contact Us</h1>
      <p className="text-lg text-muted-foreground mb-8 text-center">
        Questions about compliance or technical capabilities? Send us a message.
      </p>
      <div className="bg-card border rounded-lg p-6 shadow-sm">
        <ContactForm />
      </div>
    </div>
  );
}
