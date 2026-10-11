import { NextResponse, type NextRequest } from "next/server";
import { createSession, safeEqual, SESSION_COOKIE, SESSION_MAX_AGE } from "@/lib/session";

export async function POST(req: NextRequest) {
  const form = await req.formData();
  const password = String(form.get("password") ?? "");
  const expected = process.env.APP_PASSWORD ?? "";

  if (!expected || !safeEqual(password, expected)) {
    await new Promise((r) => setTimeout(r, 700)); // slow down guessing
    return NextResponse.redirect(new URL("/login?error=1", req.url), 303);
  }
  const res = NextResponse.redirect(new URL("/", req.url), 303);
  res.cookies.set(SESSION_COOKIE, await createSession(), {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: SESSION_MAX_AGE,
  });
  return res;
}
