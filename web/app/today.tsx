"use client";

import { useRef, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Page, Section } from "@/lib/types";
import AskBar from "@/components/AskBar";
import TopicSection, { Skeleton } from "@/components/TopicSection";

const EXAMPLE = "AI policy, substantive only";

type Pending = { kind: "new" } | { kind: "change"; topicId: number } | null;

function headerDate(iso: string): string {
  const d = new Date(iso + "T00:00:00Z");
  const weekday = d.toLocaleDateString("en-GB", { weekday: "short", timeZone: "UTC" });
  const day = d.toLocaleDateString("en-GB", { day: "numeric", month: "short", timeZone: "UTC" });
  return `${weekday} ${day}`;
}

function errorText(status: number): string {
  if (status === 429) return "That's enough new topics for today. Try again tomorrow.";
  if (status === 400) return "Try describing it in a few words.";
  return "Couldn't fetch news for that right now. Try again in a minute.";
}

async function call(path: string, method: string, body?: object): Promise<Response> {
  const res = await fetch(path, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 401) window.location.href = "/login";
  return res;
}

export default function Today({ page }: { page: Page }) {
  const router = useRouter();
  const inputRef = useRef<HTMLInputElement>(null);
  const [text, setText] = useState("");
  const [changing, setChanging] = useState<Section | null>(null);
  const [pending, setPending] = useState<Pending>(null);
  const [error, setError] = useState<string | null>(null);
  const [, startTransition] = useTransition();

  const refresh = () =>
    startTransition(() => {
      router.refresh();
      setPending(null);
    });

  async function submit(value: string) {
    setError(null);
    const target = changing;
    setPending(target ? { kind: "change", topicId: target.topicId } : { kind: "new" });
    window.scrollTo({ top: 0, behavior: "smooth" });
    const res = target
      ? await call(`/api/topics/${target.topicId}`, "PATCH", { text: value })
      : await call("/api/topics", "POST", { text: value });
    if (res.ok) {
      setText("");
      setChanging(null);
    } else {
      setError(errorText(res.status));
    }
    // A failed new topic may still have been created (with nothing found), so refresh either way.
    refresh();
  }

  function startChange(section: Section) {
    setError(null);
    setChanging(section);
    setText(section.name);
    inputRef.current?.focus();
  }

  function cancelChange() {
    setChanging(null);
    setText("");
  }

  async function follow(topicId: number) {
    const res = await call(`/api/topics/${topicId}/follow`, "POST");
    if (res.ok) refresh();
  }

  async function stop(section: Section) {
    if (!window.confirm(`Stop following ${section.name}?`)) return;
    const res = await call(`/api/topics/${section.topicId}`, "DELETE");
    if (res.ok) refresh();
  }

  const busy = pending !== null;
  const bar = (docked: boolean) => (
    <AskBar
      value={text}
      onValueChange={setText}
      onSubmit={submit}
      onCancelChange={cancelChange}
      changingName={changing?.name ?? null}
      busy={busy}
      error={error}
      inputRef={inputRef}
      docked={docked}
    />
  );

  if (page.sections.length === 0) {
    return (
      <main className="page first-run">
        <h1 className="title">What do you want to follow?</h1>
        {pending?.kind === "new" && <Skeleton />}
        {bar(false)}
        <button type="button" className="chip" onClick={() => setText(EXAMPLE)} disabled={busy}>
          {EXAMPLE}
        </button>
      </main>
    );
  }

  const stories = page.count === 1 ? "1 story" : `${page.count} stories`;
  return (
    <>
      <main className="page">
        <header className="masthead">
          <h1 className="title">{headerDate(page.date)}</h1>
          <p className="meta">
            {stories} · about {Math.max(page.readMinutes, 1)} min
          </p>
        </header>
        {pending?.kind === "new" && (
          <section className="topic" aria-label="Trying a new topic">
            <p className="trying-label">Trying</p>
            <Skeleton />
          </section>
        )}
        {page.sections.map((s) => (
          <TopicSection
            key={s.topicId}
            section={s}
            changing={pending?.kind === "change" && pending.topicId === s.topicId}
            onChange={startChange}
            onFollow={follow}
            onStop={stop}
          />
        ))}
        <p className="ending">That&apos;s today. Nothing else is queued.</p>
      </main>
      {bar(true)}
    </>
  );
}
