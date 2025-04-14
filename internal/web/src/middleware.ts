import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

export function middleware(request: NextRequest) {
  const path = request.nextUrl.pathname;

  // Public paths that don't require authentication
  const isAuthPath = path === "/auth/login" || path === "/auth/register";
  const isPublicPath = path === "/" || isAuthPath;

  // Check for your existing session cookies
  const sessionCookie = request.cookies.get("session")?.value;
  const authToken = request.cookies.get("auth_token")?.value;
  const hasAuthCookies = !!sessionCookie || !!authToken;

  // Redirect logic
  if (!hasAuthCookies && !isPublicPath) {
    return NextResponse.redirect(new URL("/auth/login", request.url));
  }

  if (hasAuthCookies && isAuthPath) {
    return NextResponse.redirect(new URL("/dashboard", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico).*)"],
};
