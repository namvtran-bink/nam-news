# Decisions

Choices the plan left open, one line each.

- Today's page shows items whose `run_date` is today (Asia/Saigon); before the 05:30 run, followed topics show "Nothing worth your time here today."
- Followed topics with zero items still get a section (with that empty line), so the page reflects what you follow.
- Search URLs already saved for the topic in the last 7 days are filtered out *before* the model call, so they cost nothing and can't reappear.
- The curator may only return URLs we gave it; an invented URL counts as invalid JSON (retry once, then save nothing).
- Tavily look-back uses `start_date` = today − 2 days (the API has no `days` parameter); `search_depth: basic`, 10 results.
- Model calls use `output_config.effort: low` and `max_tokens: 4096` (Haiku 5.5 thinks by default; thinking counts toward max_tokens).
- Anthropic is called over plain HTTP (`/v1/messages`, `anthropic-version: 2023-06-01`), no SDK, to keep the binary small.
- `TRIAL_CAP_PER_DAY` counts every topic created since local midnight (bar and `quiet run`), whatever its status now.
- If a new topic's search or curation fails, the trial topic is still saved (with no items) and the API returns 502; the owner can retry via Change.
- PATCH (Change) deletes that topic's items for today, then re-runs it under the new rules.
- `POST /api/admin/run-daily` runs synchronously and returns a report; a second call while one runs gets 409. It is not exposed through the web app.
- "Just once" is remembered in the browser's localStorage only; the topic disappears the next day anyway.
- Added a small **Stop** link on followed topics (DELETE → dropped), since "drop" is in scope but the plan gave it no control.
- Added `quiet run -follow "<text>"` to seed followed topics from the terminal.
- The page header is split into a serif date ("Sat 10 Oct") and a muted line ("11 stories · about 5 min").
- Session cookie is HMAC-signed with a key derived from `APP_TOKEN` (no extra secret to manage); 30-day expiry.
- Next.js 15.5 (App Router) with `postcss` pinned to a patched version via `overrides` (npm audit clean).
- The web unit runs `node .../next start` directly (not `npm run start`), so npm never needs a writable home under systemd's sandboxing.
- Deploy target: AWS Lightsail 2 GB (Singapore), Ubuntu 24.04, Node 22 from NodeSource, Go from snap; private repo cloned with a read-only deploy key.
- Mac mode (owner's request, 2026-10-11): no server. `mac/update.sh` runs `quiet daily` then `quiet render`, which writes one self-contained `index.html`; Cloudflare Pages (free, works with a private repo via direct upload) hosts it.
- The static page is public-by-link (noindex) with no password; Cloudflare Access can add one later if needed.
- New CLI commands for Mac mode: `add` (= `run -follow`), `topics`, `drop <id>`, `render [-o dir]`. `render`, `topics` and `drop` need no API keys.
- Mac data lives in `~/.quiet/`; the scheduled run uses launchd at 05:30 local time and catches up after sleep.
- Docker mode: one image (Go binary + wrangler on node:22-slim). `schedule` (default command) checks every 5 minutes and runs `update` once a day after $RUN_AT, so it catches up after the Mac sleeps. Cloudflare auth uses an API token, since `wrangler login` needs a browser callback that doesn't work inside a container. Data is bind-mounted to ./data so the page is visible on the Mac.
- GitHub Actions mode: daily cron `30 22 * * *` (05:30 UTC+7) plus manual add/drop/topics/update/render. The SQLite file is stored on a `data` branch as a single force-pushed commit, written with git plumbing so it reuses the checkout's credentials. A failed restore stops the run so an empty database can never overwrite the real one. Runs are serialized with a concurrency group.
