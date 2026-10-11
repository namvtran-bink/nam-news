import type { NextRequest } from "next/server";
import { proxy } from "@/lib/backend";

type Ctx = { params: Promise<{ id: string }> };

export async function PATCH(req: NextRequest, { params }: Ctx) {
  const { id } = await params;
  return proxy(`/api/topics/${encodeURIComponent(id)}`, "PATCH", await req.text());
}

export async function DELETE(_req: NextRequest, { params }: Ctx) {
  const { id } = await params;
  return proxy(`/api/topics/${encodeURIComponent(id)}`, "DELETE");
}
