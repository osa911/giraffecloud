/**
 * Application Routes
 *
 * Centralized route paths to prevent typos and make refactoring easier.
 * Use these constants throughout the application instead of hardcoded strings.
 */

export const ROUTES = {
  // Public routes
  HOME: "/",
  ABOUT: "/about",
  PRICING: "/pricing",
  CONTACT: "/contact",
  INSTALLATION: "/installation",

  // Legal pages
  TERMS: "/terms",
  PRIVACY: "/privacy",
  ACCEPTABLE_USE: "/acceptable-use",
  COOKIE_POLICY: "/cookie-policy",
  REFUND: "/refund",

  // Auth routes
  AUTH: {
    LOGIN: "/auth/login",
    REGISTER: "/auth/register",
  },

  // Dashboard routes
  DASHBOARD: {
    HOME: "/dashboard",
    GETTING_STARTED: "/dashboard/getting-started",
    TUNNELS: "/dashboard/tunnels",
    PROFILE: "/dashboard/profile",
    SETTINGS: "/dashboard/settings",
    ADMIN: "/dashboard/admin",
  },
} as const;

/**
 * Helper to check if a path is a public route
 */
export function isPublicRoute(path: string): boolean {
  const publicPaths = [
    ROUTES.HOME,
    ROUTES.AUTH.LOGIN,
    ROUTES.AUTH.REGISTER,
    ROUTES.ABOUT,
    ROUTES.PRICING,
    ROUTES.TERMS,
    ROUTES.PRIVACY,
    ROUTES.CONTACT,
    ROUTES.INSTALLATION,
    ROUTES.ACCEPTABLE_USE,
    ROUTES.COOKIE_POLICY,
    ROUTES.REFUND,
  ];

  return publicPaths.some((publicPath) => path === publicPath || path.startsWith(publicPath));
}

/**
 * Helper to check if a path is a dashboard route
 */
export function isDashboardRoute(path: string): boolean {
  return path.startsWith(ROUTES.DASHBOARD.HOME);
}

/**
 * Helper to check if a path is an auth route
 */
export function isAuthRoute(path: string): boolean {
  return path === ROUTES.AUTH.LOGIN || path === ROUTES.AUTH.REGISTER;
}

