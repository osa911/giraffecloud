import Link from "next/link";
import { ROUTES } from "@/constants/routes";
import { Separator } from "@/components/ui/separator";

export default function Footer() {
  return (
    <footer className="py-6 bg-background">
      <Separator className="mb-6" />
      <div className="container mx-auto px-4 max-w-7xl">
        <div className="flex flex-col sm:flex-row justify-between items-center gap-4 text-sm text-muted-foreground">
          <p>
            © <Link href={ROUTES.HOME} className="hover:underline">GiraffeCloud</Link> {new Date().getFullYear()}
          </p>
          <div className="flex flex-wrap gap-4">
            <Link href={ROUTES.ABOUT} className="hover:underline hover:text-foreground transition-colors">
              About
            </Link>
            <Link href={ROUTES.CONTACT} className="hover:underline hover:text-foreground transition-colors">
              Contact
            </Link>
            <Link href={ROUTES.INSTALLATION} className="hover:underline hover:text-foreground transition-colors">
              Install
            </Link>
            <Link href={ROUTES.TERMS} className="hover:underline hover:text-foreground transition-colors">
              Terms
            </Link>
            <Link href={ROUTES.PRIVACY} className="hover:underline hover:text-foreground transition-colors">
              Privacy
            </Link>
            <Link href={ROUTES.ACCEPTABLE_USE} className="hover:underline hover:text-foreground transition-colors">
              AUP
            </Link>
            <Link href={ROUTES.COOKIE_POLICY} className="hover:underline hover:text-foreground transition-colors">
              Cookies
            </Link>
          </div>
        </div>
      </div>
    </footer>
  );
}
