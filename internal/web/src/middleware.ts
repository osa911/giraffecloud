import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

export function middleware(request: NextRequest) {
  const path = request.nextUrl.pathname;

  // Public paths that don't require authentication
  const isPublicPath =
    path === "/" || path === "/auth/login" || path === "/auth/register";

  // Check for your existing session cookies
  const sessionCookie = request.cookies.get("session")?.value;
  const authToken = request.cookies.get("auth_token")?.value;
  const hasAuthCookies = !!sessionCookie || !!authToken;

  console.log("hasAuthCookies", hasAuthCookies);
  console.log("isPublicPath", isPublicPath);
  console.log("path", path);
  console.log("sessionCookie", sessionCookie);
  console.log("authToken", authToken);
  // Redirect logic
  if (!hasAuthCookies && !isPublicPath) {
    return NextResponse.redirect(new URL("/auth/login", request.url));
  }

  if (hasAuthCookies && isPublicPath && path !== "/") {
    return NextResponse.redirect(new URL("/dashboard", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico).*)"],
};
