import { Container, Typography, Box } from "@mui/material";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Terms of Service | GiraffeCloud",
  description: "The terms that govern use of GiraffeCloud services.",
};

export default function TermsPage() {
  return (
    <main>
      <Container maxWidth="md">
        <Box sx={{ py: 8 }}>
          <Typography variant="h2" component="h1" gutterBottom>
            Terms of Service
          </Typography>
          <Typography color="text.secondary">
            These Terms of Service ("Terms") govern your access to and use of the GiraffeCloud
            services (the "Service"). By using the Service, you agree to be bound by these Terms.
          </Typography>

          <Typography variant="h5" gutterBottom sx={{ mt: 4 }}>
            1. Accounts and Eligibility
          </Typography>
          <Typography>
            You must be at least 18 years old and have the legal capacity to enter into these Terms.
            You are responsible for safeguarding your account credentials.
          </Typography>

          <Typography variant="h5" gutterBottom sx={{ mt: 3 }}>
            2. Acceptable Use
          </Typography>
          <Typography>
            You may not use the Service to violate any law, infringe intellectual property rights,
            transmit malware, perform denial-of-service attacks, or abuse network resources. We may
            suspend or terminate accounts engaged in prohibited activity.
          </Typography>

          <Typography variant="h5" gutterBottom sx={{ mt: 3 }}>
            3. Fees and Billing
          </Typography>
          <Typography>
            Paid plans are billed in advance on a subscription basis. Charges are non-refundable
            except as required by law or as described in our Refund Policy.
          </Typography>

          <Typography variant="h5" gutterBottom sx={{ mt: 3 }}>
            4. Service Availability
          </Typography>
          <Typography>
            We strive for high availability but do not guarantee uninterrupted service. We may
            perform maintenance or updates that temporarily affect the Service.
          </Typography>

          <Typography variant="h5" gutterBottom sx={{ mt: 3 }}>
            5. Data and Privacy
          </Typography>
          <Typography>
            Our Privacy Policy explains how we collect and process personal data. You consent to our
            data practices by using the Service.
          </Typography>

          <Typography variant="h5" gutterBottom sx={{ mt: 3 }}>
            6. Disclaimers and Limitation of Liability
          </Typography>
          <Typography>
            The Service is provided "as is" without warranties of any kind. To the maximum extent
            permitted by law, GiraffeCloud shall not be liable for indirect or consequential
            damages.
          </Typography>

          <Typography variant="h5" gutterBottom sx={{ mt: 3 }}>
            7. Termination
          </Typography>
          <Typography>
            We may suspend or terminate your access if you breach these Terms. You may stop using
            the Service at any time.
          </Typography>

          <Typography variant="h5" gutterBottom sx={{ mt: 3 }}>
            8. Changes
          </Typography>
          <Typography>
            We may update these Terms from time to time. If changes are material, we will provide
            notice by posting an update on this page or via email.
          </Typography>

          <Typography variant="body2" color="text.secondary" sx={{ mt: 6 }}>
            Last updated: {new Date().toISOString().split("T")[0]}
          </Typography>
        </Box>
      </Container>
    </main>
  );
}
