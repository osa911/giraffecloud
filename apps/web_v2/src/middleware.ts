import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

// Paths that require authentication
const PROTECTED_PATHS = ["/dashboard"];

// Paths that are public (auth pages)
const AUTH_PATHS = ["/auth/login", "/auth/register"];

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // Check if the path is protected
  const isProtected = PROTECTED_PATHS.some((path) => pathname.startsWith(path));
  const isAuthPage = AUTH_PATHS.some((path) => pathname.startsWith(path));

  // Get the session cookie
  // Note: We're checking for "session" or "auth_token" depending on what your backend sets
  // Based on baseApiClient.ts, it seems to be "session" or "auth_token"
  const hasSession = request.cookies.has("session") || request.cookies.has("auth_token");

  if (isProtected && !hasSession) {
    // Redirect to login if accessing protected route without session
    const url = new URL("/auth/login", request.url);
    url.searchParams.set("callbackUrl", pathname);
    return NextResponse.redirect(url);
  }

  if (isAuthPage && hasSession) {
    // Redirect to dashboard if accessing auth pages while logged in
    return NextResponse.redirect(new URL("/dashboard", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: [
    /*
     * Match all request paths except for the ones starting with:
     * - api (API routes)
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * - favicon.ico (favicon file)
     * - public files (images, etc)
     */
    "/((?!api|_next/static|_next/image|favicon.ico|avatars|.*\\.(?:svg|png|jpg|jpeg|gif|webp)$).*)",
  ],
};
