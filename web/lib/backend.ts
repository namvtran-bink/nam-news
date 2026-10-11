// Server-only: talks to the Go backend with the shared bearer token.
// Never import this from a client component.
import { NextResponse } from "next/server";

const BACKEND_URL = process.env.BACKEND_URL ?? "http://127.0.0.1:8080";

export function backendFetch(path: string, init: RequestInit = {}): Promise<Response> {
  return fetch(BACKEND_URL + path, {
    ...init,
    cache: "no-store",
    headers: {
      ...init.headers,
      Authorization: `Bearer ${process.env.APP_TOKEN ?? ""}`,
      "Content-Type": "application/json",
    },
  });
}

// proxy forwards one call to the backend and relays its status and JSON body.
export async function proxy(path: string, method: string, body?: string): Promise<NextResponse> {
  try {
    const res = await backendFetch(path, { method, body });
    const data = await res.json().catch(() => ({}));
    return NextResponse.json(data, { status: res.status });
  } catch {
    return NextResponse.json({ error: "backend unreachable" }, { status: 502 });
  }
}
