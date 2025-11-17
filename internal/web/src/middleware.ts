import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { ROUTES, isDashboardRoute } from "@/constants/routes";

export function middleware(request: NextRequest) {
  const path = request.nextUrl.pathname;

  // Check for session cookies
  const sessionCookie = request.cookies.get("session")?.value;
  const authToken = request.cookies.get("auth_token")?.value;
  const hasAuthCookies = !!sessionCookie || !!authToken;

  // Only redirect to login if accessing protected routes (dashboard/*) without cookies
  if (!hasAuthCookies && isDashboardRoute(path)) {
    return NextResponse.redirect(new URL(ROUTES.AUTH.LOGIN, request.url));
  }

  // Don't redirect authenticated users away from auth pages in middleware
  // Instead, the auth pages themselves check authentication and redirect to dashboard
  // This prevents redirect loops when cookies exist but are invalid:
  // - Middleware can only check if cookies EXIST, not if they're VALID
  // - Server-side auth pages call getAuthUser() which validates cookies
  // - If valid: redirect to dashboard
  // - If invalid: clear cookies and show login page

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico|icon.png).*)"],
};
