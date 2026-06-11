import { NextRequest, NextResponse } from "next/server";

// "/" must be public so the root page can capture ?token= from the Go OAuth callback.
const PUBLIC_PATHS = ["/", "/login", "/auth"];

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const isPublic = PUBLIC_PATHS.some((p) => pathname.startsWith(p));
  if (isPublic) return NextResponse.next();

  const token = request.cookies.get("pr_reviewer_token");
  if (!token) {
    return NextResponse.redirect(new URL("/login", request.url));
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next|favicon.ico|public).*)"],
};
