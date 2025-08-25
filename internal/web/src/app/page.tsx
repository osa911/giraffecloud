import Link from "@/components/common/Link";
import { Button, Container, Typography, Box } from "@mui/material";

export default function HomeServerPage() {
  return (
    <main>
      <Container maxWidth="lg">
        <Box
          sx={{
            minHeight: "100vh",
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
            <Link href="/auth/register">
              <Button variant="contained" size="large" sx={{ mr: 2 }}>
                Get Started
              </Button>
            </Link>
            <Link href="/auth/login">
              <Button variant="outlined" size="large">
                Login
              </Button>
            </Link>
          </Box>
        </Box>
      </Container>
    </main>
  );
}
