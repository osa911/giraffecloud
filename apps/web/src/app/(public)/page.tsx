"use client";

import HomeRedirectHandler from "@/components/home/HomeRedirectHandler";
import AuthButtons from "@/components/auth/AuthButtons";
import { motion } from "framer-motion";
import { ShieldCheck, Zap, Globe, Wifi } from "lucide-react";

export default function HomePage() {
  return (
    <>
      <HomeRedirectHandler />
      <div className="flex flex-col items-center justify-center min-h-[80vh] text-center space-y-8 py-12">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5 }}
          className="space-y-4 max-w-3xl"
        >

          <h1 className="text-4xl font-extrabold tracking-tight lg:text-6xl bg-gradient-to-r from-foreground to-foreground/70 bg-clip-text text-transparent">
            Secure Tunnel Service for Your Applications
          </h1>
          <p className="text-xl text-muted-foreground max-w-[600px] mx-auto">
            Expose your local server to the internet without revealing your IP address. Works seamlessly with Dynamic IPs. Secure, fast, and user-friendly.
          </p>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.2 }}
          className="flex flex-col sm:flex-row gap-4 justify-center"
        >
          <AuthButtons />
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 40 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.7, delay: 0.4 }}
          className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-8 mt-16 text-left"
        >
          <FeatureCard
            icon={<ShieldCheck className="w-10 h-10 text-primary" />}
            title="Secure by Default"
            description="End-to-end encryption, automatic HTTPS, and IP privacy—your real address stays hidden."
          />
          <FeatureCard
            icon={<Zap className="w-10 h-10 text-primary" />}
            title="Lightning Fast"
            description="Optimized low-latency connections powered by global edge network."
          />
          <FeatureCard
            icon={<Globe className="w-10 h-10 text-primary" />}
            title="Custom Domains"
            description="Bring your own domain or use our free subdomain for your project."
          />
          <FeatureCard
            icon={<Wifi className="w-10 h-10 text-primary" />}
            title="Dynamic IP Support"
            description="Host from anywhere. We handle IP changes automatically, so you don't need a static IP."
          />
        </motion.div>
      </div>
    </>
  );
}

function FeatureCard({ icon, title, description }: { icon: React.ReactNode; title: string; description: string }) {
  return (
    <div className="p-6 rounded-xl border bg-card text-card-foreground shadow-sm hover:shadow-md transition-shadow">
      <div className="mb-4">{icon}</div>
      <h3 className="text-xl font-semibold mb-2">{title}</h3>
      <p className="text-muted-foreground">{description}</p>
    </div>
  );
}
