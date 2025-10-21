"use client";
import { Box, TextField, Button, Stack, Typography } from "@mui/material";
import { useState, useEffect } from "react";
import toast from "react-hot-toast";
import clientApi from "@/services/apiClient/clientApiClient";

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
    loadRecaptcha()
      .then(() => setRecaptchaReady(true))
      .catch((err) => {
        console.error("reCAPTCHA load error:", err);
        toast.error("Failed to load security verification");
      });
  }, []);

  const getRecaptchaToken = async (): Promise<string> => {
    return new Promise((resolve, reject) => {
      if (!recaptchaReady || !window.grecaptcha) {
        reject(new Error("reCAPTCHA not ready"));
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

      // Success - error handling is done by the API client interceptor
      toast.success(response.message || "Message sent successfully!");
      setName("");
      setEmail("");
      setMessage("");
    } catch (error) {
      // Error toasts are already shown by the API client interceptor
      // Just log it for debugging
      console.error("Contact form error:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Box component="form" onSubmit={onSubmit} noValidate>
      <Stack spacing={2}>
        <TextField
          label="Name"
          required
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={loading}
          helperText="Required (min 2 characters)"
        />
        <TextField
          label="Email"
          type="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          disabled={loading}
          helperText="Required"
        />
        <TextField
          label="Message"
          required
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          multiline
          minRows={4}
          disabled={loading}
          helperText={`${message.length}/${MAX_MESSAGE_LENGTH} characters`}
          error={message.length > MAX_MESSAGE_LENGTH}
        />
        <Button type="submit" variant="contained" disabled={loading || !recaptchaReady}>
          {loading ? "Sending..." : "Send"}
        </Button>
        <Typography variant="body2" color="text.secondary">
          This site is protected by reCAPTCHA and the Google{" "}
          <a href="https://policies.google.com/privacy" target="_blank" rel="noopener">
            Privacy Policy
          </a>{" "}
          and{" "}
          <a href="https://policies.google.com/terms" target="_blank" rel="noopener">
            Terms of Service
          </a>{" "}
          apply.
        </Typography>
      </Stack>
    </Box>
  );
}
