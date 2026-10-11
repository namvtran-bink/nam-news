import type { NextRequest } from "next/server";
import { proxy } from "@/lib/backend";

export async function POST(_req: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return proxy(`/api/topics/${encodeURIComponent(id)}/follow`, "POST");
}
