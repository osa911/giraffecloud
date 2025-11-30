import { Metadata } from "next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Check } from "lucide-react";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Pricing | GiraffeCloud",
  description: "Plans and pricing for GiraffeCloud.",
};

type Plan = {
  name: string;
  price: string;
  description: string;
  features: string[];
  ctaHref: string;
  ctaText: string;
  popular?: boolean;
};

const plans: Plan[] = [
  {
    name: "Free",
    price: "$0",
    description: "For hobbyists and personal projects.",
    features: ["1 tunnel", "1 custom subdomain", "Basic rate limits"],
    ctaHref: "/auth/register",
    ctaText: "Get Started",
  },
  {
    name: "Pro",
    price: "$12",
    description: "For professionals and small teams.",
    features: ["Up to 5 tunnels", "Custom domains", "Higher limits", "Email support"],
    ctaHref: "/auth/register",
    ctaText: "Get Started",
    popular: true,
  },
  {
    name: "Business",
    price: "Contact",
    description: "For organizations with advanced needs.",
    features: ["Unlimited tunnels", "SLA & priority support", "SAML/SSO", "Dedicated throughput"],
    ctaHref: "/contact",
    ctaText: "Contact Sales",
  },
];

export default function PricingPage() {
  return (
    <div className="container mx-auto px-4 py-16 max-w-6xl">
      <div className="text-center mb-12">
        <h1 className="text-4xl font-bold mb-4">Pricing</h1>
        <p className="text-lg text-muted-foreground">
          Simple, predictable pricing. Upgrade or cancel anytime.
        </p>
      </div>

      <div className="grid md:grid-cols-3 gap-8">
        {plans.map((plan) => (
          <Card key={plan.name} className={plan.popular ? "border-primary shadow-lg relative" : ""}>
            {plan.popular && (
              <div className="absolute -top-4 left-1/2 -translate-x-1/2 bg-primary text-primary-foreground text-sm font-medium px-3 py-1 rounded-full">
                Most Popular
              </div>
            )}
            <CardHeader>
              <CardTitle className="text-2xl">{plan.name}</CardTitle>
              <CardDescription>{plan.description}</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="mb-6">
                <span className="text-4xl font-bold">{plan.price}</span>
                {plan.price !== "Contact" && <span className="text-muted-foreground">/mo</span>}
              </div>
              <ul className="space-y-3">
                {plan.features.map((feature) => (
                  <li key={feature} className="flex items-center gap-2">
                    <Check className="h-4 w-4 text-primary" />
                    <span className="text-sm">{feature}</span>
                  </li>
                ))}
              </ul>
            </CardContent>
            <CardFooter>
              <Button asChild className="w-full" variant={plan.popular ? "default" : "outline"}>
                <Link href={plan.ctaHref}>{plan.ctaText}</Link>
              </Button>
            </CardFooter>
          </Card>
        ))}
      </div>
    </div>
  );
}
