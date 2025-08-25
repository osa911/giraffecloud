import { Container, Typography, Box, Grid, Card, CardContent, Button } from "@mui/material";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Pricing | GiraffeCloud",
  description: "Plans and pricing for GiraffeCloud.",
};

type Plan = {
  name: string;
  price: string;
  features: string[];
  ctaHref: string;
};

const plans: Plan[] = [
  {
    name: "Free",
    price: "$0/mo",
    features: ["1 tunnel", "1 custom subdomain", "Basic rate limits"],
    ctaHref: "/auth/register",
  },
  {
    name: "Pro",
    price: "$12/mo",
    features: ["Up to 5 tunnels", "Custom domains", "Higher limits", "Email support"],
    ctaHref: "/auth/register",
  },
  {
    name: "Business",
    price: "Contact",
    features: ["Unlimited tunnels", "SLA & priority support", "SAML/SSO", "Dedicated throughput"],
    ctaHref: "/contact",
  },
];

export default function PricingPage() {
  return (
    <main>
      <Container maxWidth="lg">
        <Box sx={{ py: 8 }}>
          <Typography variant="h2" component="h1" align="center" gutterBottom>
            Pricing
          </Typography>
          <Typography align="center" color="text.secondary" sx={{ mb: 6 }}>
            Simple, predictable pricing. Upgrade or cancel anytime.
          </Typography>
          <Grid container spacing={3}>
            {plans.map((plan) => (
              <Grid key={plan.name} component="div" size={{ xs: 12, md: 4 }}>
                <Card variant="outlined">
                  <CardContent>
                    <Typography variant="h5" gutterBottom>
                      {plan.name}
                    </Typography>
                    <Typography variant="h4" gutterBottom>
                      {plan.price}
                    </Typography>
                    <Box component="ul" sx={{ pl: 2, mb: 2 }}>
                      {plan.features.map((f) => (
                        <li key={f}>
                          <Typography component="span">{f}</Typography>
                        </li>
                      ))}
                    </Box>
                    <Button href={plan.ctaHref} variant="contained" fullWidth>
                      {plan.name === "Business" ? "Contact Sales" : "Get Started"}
                    </Button>
                  </CardContent>
                </Card>
              </Grid>
            ))}
          </Grid>
        </Box>
      </Container>
    </main>
  );
}
