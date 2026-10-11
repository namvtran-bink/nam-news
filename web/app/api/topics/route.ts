import type { NextRequest } from "next/server";
import { proxy } from "@/lib/backend";

export async function POST(req: NextRequest) {
  return proxy("/api/topics", "POST", await req.text());
}
