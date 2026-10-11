# Quiet — build brief for the coding agent

Working title: **Quiet**. A calm, finite news page that is ready every morning, plus one bar to follow a new topic on the fly. Single user for v1 (the owner). Goal: live on a cheap VPS today.

Read this whole file before writing code. If something is ambiguous, pick the simplest option, build it, and record the choice in `DECISIONS.md` (one line each). Do not ask the owner mid-build unless blocked.

## 1. Product in one paragraph

The owner follows a handful of topics. Every morning at 05:30 (Asia/Saigon) the backend fetches fresh news per topic, filters out noise, and writes 1–2 sentence summaries with links to the originals. Opening the app shows one finished page grouped by topic, then "That's today." A bar docked at the bottom lets the owner type a new topic in their own words; a "Trying" section for it appears at the top within ~30 seconds, with two choices: **Follow daily** or **Just once**.

## 2. Scope

**Build (v1):**
1. Daily ingest for followed topics.
2. New topic on the fly via the bar (trial topics, follow / drop).
3. Password gate. Cost guard for paid API calls.

**Do NOT build (explicitly out of scope):**
layout builder or generative UI, component library beyond the three below, sharing / public pages / SEO, forking or per-viewer overlays, multiple users or signup, RSS, page scraping / full-text extraction, video, cross-outlet clustering, "why it's here" lines, notifications, settings screens, dark mode, analytics. Leave seams for them, build none of them.

## 3. Architecture

- **Backend:** one Go binary (`backend/`). HTTP API + scheduler + SQLite. Use a pure-Go SQLite driver (`modernc.org/sqlite`, no CGO). Binds to `127.0.0.1:8080` only.
- **Frontend:** Next.js (App Router, TypeScript) in `web/`. One server-rendered page. Calls the backend server-side with a bearer token; the browser never sees the token.
- **Deploy:** one small VPS. Caddy (TLS) → Next.js on `:3000`. Backend is never exposed publicly. systemd units for both.
- **Contract between them is plain JSON** (section 6). Keep the frontend dumb: it renders what the API returns, so a different UI can be plugged in later without touching the backend.

```
repo/
  CLAUDE.md  DECISIONS.md  README.md  .env.example
  backend/
    cmd/quiet/main.go        # subcommands: serve | run "<topic text>" | daily
    internal/{db,search,llm,curate,api,job}/
  web/
    app/{page.tsx, login/, api/...}   components/{TopicSection,StoryRow,TryingBanner,AskBar}
  deploy/{Caddyfile, quiet-backend.service, quiet-web.service, backup.sh}
```

## 4. The one function everything uses

`runTopic(topic)`:
1. **Search:** one news-search call (Tavily; verify current parameter names in its docs: news topic, last ~2 days, ~10 results, with snippet content). Wrap behind a `Searcher` interface so the provider is swappable.
2. **Curate:** one LLM call (model from `MODEL` env, default `claude-haiku-5-5`). Input: topic rules + the results (title, url, source, date, snippet). Output: strict JSON (below). Validate in Go; retry once on invalid JSON; on second failure save nothing and log.
3. **Save:** upsert items. Skip URLs already saved for this topic in the last 7 days. Max **4 items per topic**.

The daily job calls `runTopic` for each `following` topic. The bar calls it for a new `trial` topic synchronously (target < 30 s).

### Prompt 1 — sentence → topic
Input: the owner's free text. Output JSON only:
```json
{"name":"Solid-state batteries","query":"solid-state battery news",
 "include":["research","industry"],"exclude":["product launches"],
 "understood":"Batteries: research and industry news, no product launches."}
```
`understood` is shown in the UI as one short line under the Trying banner.

### Prompt 2 — curate
System rules (keep these verbatim in spirit):
- Use ONLY the supplied snippets. No outside knowledge. Never fill gaps.
- Keep items that clearly match `include`; drop anything matching `exclude`, opinion pieces, reaction threads, listicles, press-release fluff, and duplicates of the same story.
- `summary`: 1–2 sentences, max ~45 words, neutral, in your own words (no copied sentences).
- If a snippet is too thin to summarize honestly, set `"summary": null` (UI shows title + source only).
- Prefer fewer, better items. Zero items is a valid answer.

Output JSON only:
```json
{"keep":[{"url":"...","summary":"... or null","reason":"matched: research"}]}
```
Store `reason` in the DB for debugging; do not show it in v1.

## 5. Data model (SQLite)

- `topics(id, name, query, include_json, exclude_json, understood, status, created_at, last_run_at)` — `status` ∈ `trial | following | dropped`.
- `items(id, topic_id, url, title, source, published_at, summary, reason, run_date, created_at)` — unique `(topic_id, url)`.
- `runs(id, topic_id, started_at, ok, error, n_results, n_kept)` — for debugging cost and failures.
- Trial topics are shown only on the day they were created. After that they are hidden unless followed.

Migrations: plain SQL files applied at startup. WAL mode on.

## 6. API (backend, all require `Authorization: Bearer $APP_TOKEN`)

- `GET  /api/today` → page JSON:
```json
{"date":"2026-10-10","readMinutes":5,"count":11,
 "sections":[{"topicId":1,"name":"AI policy","trial":false,"understood":null,
   "items":[{"title":"...","url":"...","source":"...","date":"8 Oct","summary":"... or null"}]}]}
```
  Trial sections first, then followed topics in creation order. Cap total at 15 items. `readMinutes` ≈ items × 0.5, rounded up.
- `POST /api/topics` `{text}` → parse (Prompt 1), create `trial` topic, `runTopic`, return that section. Enforce `TRIAL_CAP_PER_DAY` (default 10); return 429 beyond it.
- `POST /api/topics/{id}/follow` → status `following` (the next daily run includes it).
- `DELETE /api/topics/{id}` → status `dropped`.
- `PATCH /api/topics/{id}` `{text}` → re-run Prompt 1 with the instruction applied to the existing rules (e.g. "less company news"), then re-run `runTopic`.
- `POST /api/admin/run-daily` → run the daily job now (for testing).
- Scheduler inside `serve` (e.g. `robfig/cron/v3` with `time.LoadLocation("Asia/Saigon")`): daily at **05:30**. Run topics sequentially; one topic failing must not stop the others.

## 7. Frontend

**First run (no topics):** centered heading "What do you want to follow?", one input, one example chip. Submitting 2–3 topics (one at a time via the same bar) builds the first page live.

**Today page** (single column, max-width ~720px, generous spacing, no sidebar, no recommendations, no infinite scroll):
- Header line: `Sat 10 Oct · 11 stories · about 5 min`.
- `TopicSection`: small-caps topic name, then `StoryRow`s.
- `StoryRow`: headline (links to original, opens new tab), `source · date`, summary (omit if null).
- `TryingBanner` (trial section only): label "Trying", the `understood` line, buttons **Follow daily** and **Just once**. "Just once" simply leaves it; it disappears tomorrow.
- Section header has a small **Change** link that focuses the bar prefilled with the topic name (calls PATCH on submit).
- Ending line: `That's today. Nothing else is queued.`
- `AskBar` docked at the bottom, placeholder `Follow something new…`. Show a skeleton in the Trying slot while waiting.

**Auth:** `APP_PASSWORD` login page → signed, httpOnly cookie; Next.js middleware protects everything else. Route handlers under `web/app/api/*` proxy to the backend and attach `APP_TOKEN`.

**Look:** calm and editorial. Suggested: ground `#EDF0EE`, ink `#17201D`, muted `#56625D`, accent `#0E6F61`; serif for the page title (e.g. Newsreader), clean sans for UI. Touch targets ≥ 44px. Real `<button>`/`<a>`/`<input>` elements with labels. Works at phone width.

## 8. Config (`.env.example`, never commit real values)

`TAVILY_API_KEY`, `ANTHROPIC_API_KEY`, `MODEL=claude-haiku-5-5`, `APP_TOKEN` (random, shared by web↔backend), `APP_PASSWORD`, `DB_PATH=/var/lib/quiet/quiet.db`, `TZ=Asia/Saigon`, `TRIAL_CAP_PER_DAY=10`, `BACKEND_URL=http://127.0.0.1:8080`.
Never log secrets. Check current SDK/API docs for Tavily and Anthropic rather than assuming parameter names.

## 9. Build order and checkpoints (stop and verify at each)

1. **Core, terminal only.** Schema + `Searcher` + `llm` + `curate` + `runTopic`. *Checkpoint:* `./quiet run "AI policy, substantive only"` prints ≤ 4 sensible items with summaries and links.
2. **API + scheduler + auth** and systemd unit. *Checkpoint:* `curl -H "Authorization: Bearer …" localhost:8080/api/today` returns valid JSON; `POST /api/admin/run-daily` works.
3. **Frontend:** login, Today page, AskBar, Trying flow, Follow/Just once. *Checkpoint:* typing a new topic shows a Trying section in < 30 s.
4. **Deploy:** Caddy TLS (owner's domain, or `<ip>.sslip.io`), both systemd units, `deploy/backup.sh` (daily `sqlite3 "$DB_PATH" ".backup …"`, keep 7). Seed the owner's real topics by running each once.

## 10. Quality bar

- Go: table-driven tests for JSON validation and dedupe using a fake `Searcher` and fake LLM; no network in tests. `go vet` clean.
- Web: typecheck passes; no unused code; no client-side secrets.
- Honesty: summaries come only from supplied snippets; always link the original; never present a null summary as if it had one.
- Cost guard: trial cap enforced server-side; password gate on every route; backend not publicly reachable.
- Keep it small. If a change adds a feature that is not in section 2 "Build", don't make it.

## 11. Done means

URL works over HTTPS behind the password; the owner's topics are seeded; typing a new topic yields a Trying section in under 30 seconds; Follow daily / Just once behave as described; the 05:30 job has been proven by one manual `run-daily`; backup script runs. Write `README.md` with run/deploy steps and list any deviations in `DECISIONS.md`.
