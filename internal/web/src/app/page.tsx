import Footer from "@/components/common/Footer";
import Link from "@/components/common/Link";
import { ROUTES } from "@/constants/routes";
import { Box, Button, Container, Typography } from "@mui/material";

export default function HomeServerPage() {
  const footerHeight = 113;
  return (
    <main>
      <Container maxWidth="lg">
        <Box
          sx={{
            minHeight: `calc(100vh - ${footerHeight}px)`,
            display: "flex",
            flexDirection: "column",
            justifyContent: "center",
            alignItems: "center",
            textAlign: "center",
            py: 8,
          }}
        >
          <Typography variant="h1" component="h1" gutterBottom>
            Welcome to GiraffeCloud
          </Typography>
          <Typography variant="h5" component="h2" gutterBottom color="text.secondary">
            Secure and efficient reverse tunnel service
          </Typography>
          <Box sx={{ mt: 4 }}>
            <Link href={ROUTES.AUTH.REGISTER}>
              <Button variant="contained" size="large" sx={{ mr: 2 }}>
                Get Started
              </Button>
            </Link>
            <Link href={ROUTES.AUTH.LOGIN}>
              <Button variant="outlined" size="large">
                Login
              </Button>
            </Link>
          </Box>
        </Box>
      </Container>
      <Footer />
    </main>
  );
}
