import Link from "next/link";
import Footer from "@/components/common/Footer";
import { ModeToggle } from "@/components/mode-toggle";
import { Cloud } from "lucide-react";
import AuthButtons from "@/components/auth/AuthButtons";

type PublicLayoutProps = {
  children: React.ReactNode;
};

export default function PublicLayout({ children }: PublicLayoutProps) {
  return (
    <div className="flex flex-col min-h-screen">
      <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="container mx-auto px-4 max-w-7xl flex h-14 items-center">
          <div className="mr-4 flex">
            <Link href="/" className="mr-6 flex items-center space-x-2">
              <Cloud className="h-6 w-6" />
              <span className="hidden font-bold sm:inline-block">
                <span className="font-extrabold text-primary">Giraffe</span>
                <span className="font-semibold">Cloud</span>
              </span>
            </Link>
            <nav className="hidden md:flex items-center space-x-6 text-sm font-medium">
              <Link href="/about" className="transition-colors hover:text-foreground/80 text-foreground/60">
                About
              </Link>
              {/* <Link href="/pricing" className="transition-colors hover:text-foreground/80 text-foreground/60">
                Pricing
              </Link> */}
              <Link href="/installation" className="transition-colors hover:text-foreground/80 text-foreground/60">
                Install
              </Link>
            </nav>
          </div>
          <div className="flex flex-1 items-center justify-between space-x-2 md:justify-end">
            <div className="w-full flex-1 md:w-auto md:flex-none">
              {/* Add search here if needed */}
            </div>
            <nav className="flex items-center gap-2">
              <ModeToggle />
              <AuthButtons />
            </nav>
          </div>
        </div>
      </header>
      <main className="flex-1 flex flex-col">
        <div className="container mx-auto px-4 max-w-7xl flex-1 flex flex-col">
          {children}
        </div>
      </main>
      <Footer />
    </div>
  );
}
