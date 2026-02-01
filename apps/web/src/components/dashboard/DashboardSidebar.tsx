"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import {
  LayoutDashboard,
  Rocket,
  ArrowLeftRight,
  User as UserIcon,
  Settings,
  Menu,
  ShieldCheck,
} from "lucide-react";
import { useState } from "react";
import { ROUTES } from "@/constants/routes";
import { User } from "@/lib/actions/user.types";

interface SidebarProps extends React.HTMLAttributes<HTMLDivElement> {
  user?: User | null;
}

export function DashboardSidebar({ className, user }: SidebarProps) {
  const pathname = usePathname();

  const items: { title: string; href: string; icon: any }[] = [
    {
      title: "Dashboard",
      href: ROUTES.DASHBOARD.HOME,
      icon: LayoutDashboard,
    },
    {
      title: "Getting Started",
      href: ROUTES.DASHBOARD.GETTING_STARTED,
      icon: Rocket,
    },
    {
      title: "Tunnels",
      href: ROUTES.DASHBOARD.TUNNELS,
      icon: ArrowLeftRight,
    },
    {
      title: "Profile",
      href: ROUTES.DASHBOARD.PROFILE,
      icon: UserIcon,
    },
    {
      title: "Settings",
      href: ROUTES.DASHBOARD.SETTINGS,
      icon: Settings,
    },
  ];

  // Add Admin link if user is admin
  if (user?.role === "admin") {
    items.push({
      title: "Admin",
      href: ROUTES.DASHBOARD.ADMIN,
      icon: ShieldCheck,
    });
  }

  return (
    <div className={cn("pb-12", className)}>
      <div className="space-y-4 py-4">
        <div className="px-3 py-2">
          <h2 className="mb-2 px-4 text-lg font-semibold tracking-tight">
            GiraffeCloud
          </h2>
          <div className="space-y-1">
            {items.map((item) => (
              <Button
                key={item.href}
                variant={pathname === item.href ? "default" : "ghost"}
                className="w-full justify-start"
                asChild
              >
                <Link href={item.href}>
                  <item.icon className="mr-2 h-4 w-4" />
                  {item.title}
                </Link>
              </Button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

export function MobileSidebar({ user }: { user?: User | null }) {
  const [open, setOpen] = useState(false);

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <Button
          variant="ghost"
          className="mr-2 px-0 text-base hover:bg-transparent focus-visible:bg-transparent focus-visible:ring-0 focus-visible:ring-offset-0 md:hidden"
        >
          <Menu className="h-6 w-6" />
          <span className="sr-only">Toggle Menu</span>
        </Button>
      </SheetTrigger>
      <SheetContent side="left" className="pr-0">
        <DashboardSidebar className="pt-4" user={user} />
      </SheetContent>
    </Sheet>
  );
}
