import Link from "@/components/common/Link";
import { Container, Typography, Box } from "@mui/material";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "About | GiraffeCloud",
  description:
    "Learn about GiraffeCloud and our mission to provide secure, reliable tunneling and reverse proxy services.",
};

export default function AboutPage() {
  return (
    <main>
      <Container maxWidth="md">
        <Box sx={{ py: 8 }}>
          <Typography variant="h2" component="h1" gutterBottom>
            About GiraffeCloud
          </Typography>
          <Typography paragraph color="text.secondary">
            GiraffeCloud is a secure tunneling and reverse proxy platform designed to help
            developers and businesses expose services safely, reliably, and at scale. We focus on
            performance, security, and operational simplicity.
          </Typography>
          <Typography variant="h5" sx={{ mt: 4 }} gutterBottom>
            What we do
          </Typography>
          <Typography component="ul" sx={{ pl: 2 }}>
            <li>
              <Typography component="span">
                Secure tunnels for HTTP and WebSocket traffic with robust authentication and access
                controls.
              </Typography>
            </li>
            <li>
              <Typography component="span">
                Automatic certificate management and modern TLS.
              </Typography>
            </li>
            <li>
              <Typography component="span">
                Reliability features: health checks, graceful deploys, and observability.
              </Typography>
            </li>
          </Typography>
          <Typography variant="h5" sx={{ mt: 4 }} gutterBottom>
            Get in touch
          </Typography>
          <Typography paragraph>
            Questions or feedback? Visit our <Link href="/contact">Contact</Link> page.
          </Typography>
        </Box>
      </Container>
    </main>
  );
}
