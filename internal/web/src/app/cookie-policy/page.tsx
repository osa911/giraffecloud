import { Container, Typography, Box } from "@mui/material";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Cookie Policy | GiraffeCloud",
  description: "How GiraffeCloud uses cookies and similar technologies.",
};

export default function CookiePolicyPage() {
  return (
    <main>
      <Container maxWidth="md">
        <Box sx={{ py: 8 }}>
          <Typography variant="h2" component="h1" gutterBottom>
            Cookie Policy
          </Typography>
          <Typography paragraph color="text.secondary">
            We use cookies and similar technologies to provide core functionality (e.g.,
            authentication), remember preferences, and measure product usage.
          </Typography>
          <Typography variant="h5" gutterBottom sx={{ mt: 4 }}>
            Types of Cookies
          </Typography>
          <Typography component="ul" sx={{ pl: 2 }}>
            <li>
              <Typography component="span">
                Strictly necessary: required for login and security.
              </Typography>
            </li>
            <li>
              <Typography component="span">
                Preferences: remember settings such as theme.
              </Typography>
            </li>
            <li>
              <Typography component="span">
                Analytics: help us understand usage and improve the Service.
              </Typography>
            </li>
          </Typography>
          <Typography paragraph sx={{ mt: 3 }}>
            You can control cookies through your browser settings. Disabling certain cookies may
            impact functionality.
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 6 }}>
            Last updated: {new Date().toISOString().split("T")[0]}
          </Typography>
        </Box>
      </Container>
    </main>
  );
}
