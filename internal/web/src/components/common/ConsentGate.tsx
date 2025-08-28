"use client";
import { PropsWithChildren, useEffect, useMemo, useState } from "react";

const CONSENT_COOKIE_NAME = "gc_cookie_consent";

function readConsent(): "accepted" | "rejected" | null {
  if (typeof document === "undefined") return null;
  const match = document.cookie.match(new RegExp(`${CONSENT_COOKIE_NAME}=([^;]+)`));
  return match ? (decodeURIComponent(match[1]) as "accepted" | "rejected") : null;
}

export default function ConsentGate({ children }: PropsWithChildren) {
  const [consent, setConsent] = useState<"accepted" | "rejected" | null>(null);
  useEffect(() => {
    setConsent(readConsent());
  }, []);

  const allowed = useMemo(() => consent === "accepted", [consent]);

  if (!allowed) return null;
  return <>{children}</>;
}
