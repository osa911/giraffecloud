import Link from "@/components/common/Link";
import { Box, Container, Divider, Stack, Typography } from "@mui/material";

export default function Footer() {
  return (
    <Box component="footer" sx={{ mt: 8, pb: 6, pt: 4, bgcolor: "background.default" }}>
      <Divider sx={{ mb: 3 }} />
      <Container maxWidth="lg">
        <Stack
          direction={{ xs: "column", sm: "row" }}
          spacing={2}
          justifyContent="space-between"
          alignItems={{ xs: "flex-start", sm: "center" }}
        >
          <Typography variant="body2" color="text.secondary">
            © {new Date().getFullYear()} GiraffeCloud
          </Typography>
          <Stack direction="row" spacing={2} flexWrap="wrap" useFlexGap>
            <Link href="/about" underline="hover">
              About
            </Link>
            <Link href="/pricing" underline="hover">
              Pricing
            </Link>
            <Link href="/contact" underline="hover">
              Contact
            </Link>
            <Link href="/installation" underline="hover">
              Install
            </Link>
            <Link href="/terms" underline="hover">
              Terms
            </Link>
            <Link href="/privacy" underline="hover">
              Privacy
            </Link>
            <Link href="/acceptable-use" underline="hover">
              AUP
            </Link>
            <Link href="/refund" underline="hover">
              Refunds
            </Link>
            <Link href="/cookie-policy" underline="hover">
              Cookies
            </Link>
          </Stack>
        </Stack>
      </Container>
    </Box>
  );
}
