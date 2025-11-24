"use client";
import { useEffect, useState } from "react";
import { Box, Button, Container, Typography, Stack, Paper } from "@mui/material";
import Link from "@/components/common/Link";

const CONSENT_COOKIE_NAME = "gc_cookie_consent";

function readConsent(): "accepted" | "rejected" | null {
  if (typeof document === "undefined") return null;
  const match = document.cookie.match(new RegExp(`${CONSENT_COOKIE_NAME}=([^;]+)`));
  if (!match || !match[1]) return null;
  return decodeURIComponent(match[1]) as "accepted" | "rejected";
}

function writeConsent(value: "accepted" | "rejected") {
  if (typeof document === "undefined") return;
  const expires = new Date();
  expires.setMonth(expires.getMonth() + 6);
  document.cookie = `${CONSENT_COOKIE_NAME}=${encodeURIComponent(value)}; path=/; expires=${expires.toUTCString()}; SameSite=Lax`;
}

export default function CookieBanner() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const current = readConsent();
    if (!current) setVisible(true);
  }, []);

  const handleAccept = () => {
    writeConsent("accepted");
    setVisible(false);
  };

  const handleReject = () => {
    writeConsent("rejected");
    setVisible(false);
  };

  if (!visible) return null;

  return (
    <Box
      sx={{
        position: "fixed",
        bottom: 16,
        left: 0,
        right: 0,
        zIndex: (theme) => theme.zIndex.snackbar,
      }}
      role="dialog"
      aria-label="Cookie consent"
    >
      <Container maxWidth="md">
        <Paper elevation={8} sx={{ p: 2 }}>
          <Stack
            direction={{ xs: "column", sm: "row" }}
            spacing={2}
            alignItems={{ xs: "flex-start", sm: "center" }}
            justifyContent="space-between"
          >
            <Typography variant="body2">
              We use cookies for essential functionality and to understand usage. See our{" "}
              <Link href="/cookie-policy">Cookie Policy</Link> and{" "}
              <Link href="/privacy">Privacy Policy</Link>.
            </Typography>
            <Stack direction="row" spacing={1}>
              <Button variant="outlined" size="small" onClick={handleReject}>
                Reject
              </Button>
              <Button variant="contained" size="small" onClick={handleAccept}>
                Accept
              </Button>
            </Stack>
          </Stack>
        </Paper>
      </Container>
    </Box>
  );
}
