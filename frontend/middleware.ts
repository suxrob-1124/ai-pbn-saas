import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

export function middleware(req: NextRequest) {
  const { pathname } = req.nextUrl;
  const access = req.cookies.get("access_token");

  const isLoggedIn = !!access?.value;

  if (pathname === "/dashboard") {
    return NextResponse.redirect(new URL("/projects", req.url));
  }

  if (pathname.startsWith("/login") && isLoggedIn) {
    return NextResponse.redirect(new URL("/me", req.url));
  }

  if (pathname.startsWith("/me") && !isLoggedIn) {
    return NextResponse.redirect(new URL("/login", req.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/", "/me/:path*", "/projects/:path*", "/monitoring/:path*", "/login", "/dashboard"]
};
