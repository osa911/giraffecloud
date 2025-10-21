import { Typography, Box, Container } from "@mui/material";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Refund Policy | GiraffeCloud",
  description: "Refund terms for GiraffeCloud paid plans.",
};

export default function RefundPolicyPage() {
  return (
    <Container maxWidth="md">
      <Box sx={{ py: 8 }}>
        <Typography variant="h2" component="h1" gutterBottom>
          Refund Policy
        </Typography>
        <Typography color="text.secondary">
          Subscriptions are billed in advance and are non-refundable, except where required by law
          or if we fail to deliver the core service for a prolonged period not caused by you or
          third-party dependencies.
        </Typography>
        <Typography variant="h5" gutterBottom sx={{ mt: 4 }}>
          Cancellations
        </Typography>
        <Typography>
          You can cancel any time. Your subscription remains active until the end of the current
          billing period.
        </Typography>
        <Typography variant="h5" gutterBottom sx={{ mt: 3 }}>
          Billing Disputes
        </Typography>
        <Typography>
          If you believe there is an error, contact support within 30 days of the charge. We will
          review and, if appropriate, issue a credit.
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 6 }}>
          Last updated: {new Date().toISOString().split("T")[0]}
        </Typography>
      </Box>
    </Container>
  );
}
