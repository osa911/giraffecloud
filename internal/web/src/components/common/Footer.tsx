import Link from "@/components/common/Link";
import { ROUTES } from "@/constants/routes";
import { Box, Container, Divider, Stack, Typography } from "@mui/material";

export default function Footer() {
  return (
    <Box component="footer" sx={{ pt: 4, pb: 4, bgcolor: "background.default" }}>
      <Divider sx={{ mb: 3 }} />
      <Container maxWidth="lg">
        <Stack
          direction={{ xs: "column", sm: "row" }}
          spacing={2}
          justifyContent="space-between"
          alignItems={{ xs: "flex-start", sm: "center" }}
        >
          <Typography variant="body2" color="text.secondary">
            © <Link href={ROUTES.HOME}>GiraffeCloud</Link> {new Date().getFullYear()}
          </Typography>
          <Stack direction="row" spacing={2} flexWrap="wrap" useFlexGap>
            <Link href={ROUTES.ABOUT} underline="hover">
              About
            </Link>
            {/* <Link href={ROUTES.PRICING} underline="hover">
              Pricing
            </Link> */}
            <Link href={ROUTES.CONTACT} underline="hover">
              Contact
            </Link>
            <Link href={ROUTES.INSTALLATION} underline="hover">
              Install
            </Link>
            <Link href={ROUTES.TERMS} underline="hover">
              Terms
            </Link>
            <Link href={ROUTES.PRIVACY} underline="hover">
              Privacy
            </Link>
            <Link href={ROUTES.ACCEPTABLE_USE} underline="hover">
              AUP
            </Link>
            {/* <Link href={ROUTES.REFUND} underline="hover">
              Refunds
            </Link> */}
            <Link href={ROUTES.COOKIE_POLICY} underline="hover">
              Cookies
            </Link>
          </Stack>
        </Stack>
      </Container>
    </Box>
  );
}
