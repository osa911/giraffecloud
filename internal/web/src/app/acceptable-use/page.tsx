import { Container, Typography, Box } from "@mui/material";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Acceptable Use Policy | GiraffeCloud",
  description: "Rules for acceptable use of GiraffeCloud services.",
};

export default function AcceptableUsePage() {
  return (
    <main>
      <Container maxWidth="md">
        <Box sx={{ py: 8 }}>
          <Typography variant="h2" component="h1" gutterBottom>
            Acceptable Use Policy
          </Typography>
          <Typography paragraph color="text.secondary">
            To keep our network safe and reliable for everyone, you agree not to misuse the Service.
            Prohibited uses include:
          </Typography>
          <Typography component="ul" sx={{ pl: 2 }}>
            <li>
              <Typography component="span">
                Illegal content or activity, including infringement and distribution of malware.
              </Typography>
            </li>
            <li>
              <Typography component="span">
                Denial-of-service attacks, port scanning at scale, or evasion of abuse detection.
              </Typography>
            </li>
            <li>
              <Typography component="span">
                Sending spam or abusive traffic; crypto mining; child sexual abuse material (CSAM).
              </Typography>
            </li>
          </Typography>
          <Typography paragraph sx={{ mt: 3 }}>
            We reserve the right to suspend or terminate accounts that violate this policy.
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 6 }}>
            Last updated: {new Date().toISOString().split("T")[0]}
          </Typography>
        </Box>
      </Container>
    </main>
  );
}
