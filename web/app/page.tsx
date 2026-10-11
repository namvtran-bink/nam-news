import { backendFetch } from "@/lib/backend";
import type { Page } from "@/lib/types";
import Today from "./today";

export const dynamic = "force-dynamic";

async function loadPage(): Promise<Page | null> {
  try {
    const res = await backendFetch("/api/today");
    return res.ok ? ((await res.json()) as Page) : null;
  } catch {
    return null;
  }
}

export default async function Home() {
  const page = await loadPage();
  if (!page) {
    return (
      <main className="page">
        <p className="ending">Today&apos;s page isn&apos;t available right now. Try again in a minute.</p>
      </main>
    );
  }
  return <Today page={page} />;
}
