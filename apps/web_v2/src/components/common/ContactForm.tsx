"use client";

import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import clientApi from "@/services/apiClient/clientApiClient";
import { Loader2 } from "lucide-react";

// Type definitions for Google reCAPTCHA
interface ReCaptchaV3 {
  ready: (callback: () => void) => void;
  execute: (siteKey: string, options: { action: string }) => Promise<string>;
}

declare global {
  interface Window {
    grecaptcha?: ReCaptchaV3;
  }
}

// Load reCAPTCHA script
const loadRecaptcha = () => {
  return new Promise<void>((resolve, reject) => {
    if (typeof window !== "undefined" && window.grecaptcha) {
      resolve();
      return;
    }

    const script = document.createElement("script");
    script.src = `https://www.google.com/recaptcha/api.js?render=${process.env.NEXT_PUBLIC_RECAPTCHA_SITE_KEY}`;
    script.async = true;
    script.defer = true;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error("Failed to load reCAPTCHA"));
    document.head.appendChild(script);
  });
};

export default function ContactForm() {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(false);
  const [recaptchaReady, setRecaptchaReady] = useState(false);

  const MAX_MESSAGE_LENGTH = 1000;

  useEffect(() => {
    if (process.env.NEXT_PUBLIC_RECAPTCHA_SITE_KEY) {
      loadRecaptcha()
        .then(() => setRecaptchaReady(true))
        .catch((err) => {
          console.error("reCAPTCHA load error:", err);
          toast.error("Failed to load security verification");
        });
    } else {
      console.warn("NEXT_PUBLIC_RECAPTCHA_SITE_KEY is not set");
    }
  }, []);

  const getRecaptchaToken = async (): Promise<string> => {
    return new Promise((resolve, reject) => {
      if (!recaptchaReady || !window.grecaptcha) {
        // If reCAPTCHA is not configured/loaded, we might want to skip or fail
        // For now, let's reject if it was expected
        if (process.env.NEXT_PUBLIC_RECAPTCHA_SITE_KEY) {
            reject(new Error("reCAPTCHA not ready"));
        } else {
            resolve("mock-token"); // Dev mode fallback
        }
        return;
      }

      window.grecaptcha.ready(() => {
        window
          .grecaptcha!.execute(process.env.NEXT_PUBLIC_RECAPTCHA_SITE_KEY!, { action: "contact" })
          .then((token: string) => resolve(token))
          .catch((err: Error) => reject(err));
      });
    });
  };

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // Validation
    if (!name || !email || !message) {
      toast.error("Please fill in all required fields.");
      return;
    }

    if (name.length < 2) {
      toast.error("Name must be at least 2 characters.");
      return;
    }

    if (message.length > MAX_MESSAGE_LENGTH) {
      toast.error(`Message is too long. Maximum ${MAX_MESSAGE_LENGTH} characters.`);
      return;
    }

    if (message.length < 10) {
      toast.error("Message is too short. Minimum 10 characters.");
      return;
    }

    setLoading(true);
    try {
      // Get reCAPTCHA token
      const recaptchaToken = await getRecaptchaToken();

      // Send to backend using API client
      const response = await clientApi().post<{ message: string; success: boolean }>(
        "/contact/submit",
        {
          name,
          email,
          message,
          recaptcha_token: recaptchaToken,
        },
      );

      // Success
      toast.success(response.message || "Message sent successfully!");
      setName("");
      setEmail("");
      setMessage("");
    } catch (error) {
      console.error("Contact form error:", error);
      // Error handling is mostly done by the API client interceptor, but we catch here to stop loading state
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="name">Name</Label>
        <Input
          id="name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={loading}
          placeholder="Your name"
          required
          minLength={2}
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="email">Email</Label>
        <Input
          id="email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          disabled={loading}
          placeholder="your@email.com"
          required
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="message">Message</Label>
        <Textarea
          id="message"
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          disabled={loading}
          placeholder="How can we help you?"
          required
          rows={5}
          className={message.length > MAX_MESSAGE_LENGTH ? "border-destructive" : ""}
        />
        <div className="flex justify-between text-xs text-muted-foreground">
          <span>Minimum 10 characters</span>
          <span className={message.length > MAX_MESSAGE_LENGTH ? "text-destructive" : ""}>
            {message.length}/{MAX_MESSAGE_LENGTH}
          </span>
        </div>
      </div>

      <Button type="submit" className="w-full" disabled={loading || (!recaptchaReady && !!process.env.NEXT_PUBLIC_RECAPTCHA_SITE_KEY)}>
        {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
        {loading ? "Sending..." : "Send Message"}
      </Button>

      <p className="text-xs text-muted-foreground text-center mt-4">
        This site is protected by reCAPTCHA and the Google{" "}
        <a href="https://policies.google.com/privacy" target="_blank" rel="noopener noreferrer" className="underline hover:text-foreground">
          Privacy Policy
        </a>{" "}
        and{" "}
        <a href="https://policies.google.com/terms" target="_blank" rel="noopener noreferrer" className="underline hover:text-foreground">
          Terms of Service
        </a>{" "}
        apply.
      </p>
    </form>
  );
}
