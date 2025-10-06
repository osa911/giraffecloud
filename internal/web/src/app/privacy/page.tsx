import { Container, Typography, Box } from "@mui/material";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Privacy Policy | GiraffeCloud",
  description: "How GiraffeCloud collects, uses, and protects your data.",
};

export default function PrivacyPolicyPage() {
  return (
    <main>
      <Container maxWidth="md">
        <Box sx={{ py: 8 }}>
          <Typography variant="h2" component="h1" gutterBottom>
            Privacy Policy
          </Typography>
          <Typography paragraph color="text.secondary">
            This Privacy Policy describes how GiraffeCloud (&quot;we&quot;, &quot;us&quot;) collects and uses personal
            information in connection with our services.
          </Typography>

          <Typography variant="h5" gutterBottom sx={{ mt: 4 }}>
            Information We Collect
          </Typography>
          <Typography component="ul" sx={{ pl: 2 }}>
            <li>
              <Typography component="span">
                Account data: name, email, authentication identifiers.
              </Typography>
            </li>
            <li>
              <Typography component="span">
                Service metadata: IP addresses, device and browser information, usage analytics.
              </Typography>
            </li>
            <li>
              <Typography component="span">
                Billing data for paid plans processed via our payment provider.
              </Typography>
            </li>
          </Typography>

          <Typography variant="h5" gutterBottom sx={{ mt: 3 }}>
            How We Use Information
          </Typography>
          <Typography component="ul" sx={{ pl: 2 }}>
            <li>
              <Typography component="span">Provide and improve the Service</Typography>
            </li>
            <li>
              <Typography component="span">
                Secure our platform, prevent abuse, and troubleshoot issues
              </Typography>
            </li>
            <li>
              <Typography component="span">Process payments and manage subscriptions</Typography>
            </li>
          </Typography>

          <Typography variant="h5" gutterBottom sx={{ mt: 3 }}>
            Data Sharing
          </Typography>
          <Typography paragraph>
            We share data with vendors who help us operate the Service (e.g., cloud hosting,
            analytics, payments). We do not sell personal information.
          </Typography>

          <Typography variant="h5" gutterBottom sx={{ mt: 3 }}>
            Your Choices
          </Typography>
          <Typography paragraph>
            You may access, correct, or delete certain personal information. You may opt out of
            marketing communications. Some functional communications are required for service
            delivery.
          </Typography>

          <Typography variant="body2" color="text.secondary" sx={{ mt: 6 }}>
            Last updated: {new Date().toISOString().split("T")[0]}
          </Typography>
        </Box>
      </Container>
    </main>
  );
}
