import Link from "@/components/common/Link";
import { ROUTES } from "@/constants/routes";
import { Box, Button, Typography } from "@mui/material";
import { getCachedUser } from "@/lib/actions/auth.actions";
import { redirect } from "next/navigation";

export default async function HomeServerPage() {
  // Get user from cache (no API call)
  const user = await getCachedUser();

  // Redirect logged-in users to dashboard
  // This is standard SaaS behavior - homepage is for marketing only
  if (user) {
    redirect(ROUTES.DASHBOARD.HOME);
  }

  return (
    <Box
      sx={{
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
  );
}
