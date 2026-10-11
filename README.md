# Quiet

A calm, finite news page, ready every morning at 05:30 (Asia/Saigon), plus one bar to follow a new topic on the fly. See `CLAUDE.md` for the product brief and `DECISIONS.md` for choices made while building.

```
backend/   Go: API + scheduler + SQLite (binds 127.0.0.1:8080)
web/       Next.js: login, Today page, proxy routes (binds 127.0.0.1:3000)
deploy/    Caddyfile, systemd units, backup script
```

## Run on your Mac (simplest setup)

No server needed. Your Mac fetches the news and builds one static page; Cloudflare Pages hosts it for free.

**One-time setup** (Terminal):

```sh
brew install go node
git clone git@github.com:namvtran-bink/nam-news.git ~/nam-news && cd ~/nam-news
cp mac/env.example mac/.env
open -e mac/.env                          # paste your Tavily and Anthropic keys; set CF_PAGES_PROJECT=nam-news

# Free Cloudflare account, then log in and create the site once
npx --yes wrangler@4 login
npx --yes wrangler@4 pages project create nam-news --production-branch main
```

**Topics** (each `add` runs once right away, then daily):

```sh
mac/quiet add "AI policy, substantive only"
mac/quiet topics
mac/quiet drop 3
```

**Update the page** whenever you like:

```sh
mac/update.sh               # fetch news, build, publish to https://nam-news.pages.dev
mac/update.sh --no-fetch    # rebuild and publish without fetching
```

**Every morning automatically** (05:30 Mac time; if the Mac is asleep, it runs when it wakes):

```sh
mac/schedule.sh on          # mac/schedule.sh off to stop; log in ~/.quiet/update.log
```

Data lives in `~/.quiet/` (database, built binary, the site). The published page is public to anyone with the link and is marked noindex.

## Run locally (server version)

Needs Go 1.26+, Node 22+, a Tavily key and an Anthropic key.

```sh
cp .env.example .env    # fill in keys; APP_TOKEN=$(openssl rand -hex 32)
set -a; . ./.env; set +a
export DB_PATH=./quiet.db

# Core check (checkpoint 1): turn a sentence into a topic and run it once
cd backend && go run ./cmd/quiet run "AI policy, substantive only"

# API + scheduler (checkpoint 2)
go run ./cmd/quiet serve
curl -H "Authorization: Bearer $APP_TOKEN" localhost:8080/api/today
curl -X POST -H "Authorization: Bearer $APP_TOKEN" localhost:8080/api/admin/run-daily

# Web (checkpoint 3), in another shell with the same env
cd web && npm ci && npm run dev    # http://localhost:3000
```

Tests (no network): `cd backend && go vet ./... && go test ./...`; `cd web && npm run typecheck`.

## Deploy

The full walkthrough, with costs, is the "Quiet: deploy plan and costs" doc in the project. In short, on an Ubuntu 24.04 server (AWS Lightsail, 2 GB):

```sh
# Tools
sudo apt update && sudo apt install -y caddy sqlite3 git
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash - && sudo apt install -y nodejs
sudo snap install go --classic            # go.mod needs Go 1.26+

# Code (private repo: add ~/.ssh/nam-news.pub as a read-only deploy key on GitHub first)
ssh-keygen -t ed25519 -N "" -f ~/.ssh/nam-news -C quiet-server
GIT_SSH_COMMAND="ssh -i ~/.ssh/nam-news -o StrictHostKeyChecking=accept-new" git clone git@github.com:namvtran-bink/nam-news.git
sudo mv nam-news /opt/quiet
git -C /opt/quiet config core.sshCommand "ssh -i /home/ubuntu/.ssh/nam-news"

# Config
sudo useradd --system --home /var/lib/quiet --shell /usr/sbin/nologin quiet
sudo mkdir -p /etc/quiet && sudo cp /opt/quiet/.env.example /etc/quiet/quiet.env
sudo chown root:quiet /etc/quiet/quiet.env && sudo chmod 640 /etc/quiet/quiet.env
sudo nano /etc/quiet/quiet.env            # keys, APP_PASSWORD, APP_TOKEN=$(openssl rand -hex 32)

# Build
mkdir -p /opt/quiet/bin
cd /opt/quiet/backend && go build -o /opt/quiet/bin/quiet ./cmd/quiet
cd /opt/quiet/web && npm ci && npm run build

# Services
sudo cp /opt/quiet/deploy/quiet-*.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now quiet-backend quiet-web

# HTTPS (your domain, or <ip>.sslip.io)
sed "s/quiet.example.com/YOUR-IP.sslip.io/" /opt/quiet/deploy/Caddyfile | sudo tee /etc/caddy/Caddyfile
sudo systemctl reload caddy

# Backups, daily at 04:15, keeps 7
echo '15 4 * * * quiet /opt/quiet/deploy/backup.sh' | sudo tee /etc/cron.d/quiet-backup

# Prove the daily job once
sudo -u quiet bash -c 'set -a; . /etc/quiet/quiet.env; curl -s -X POST -H "Authorization: Bearer $APP_TOKEN" localhost:8080/api/admin/run-daily'
```

Seed topics from the web page (type a topic, then Follow daily), or with `quiet run -follow "<text>"`.

Only ports 22, 80 and 443 need to be open; the backend and Next.js listen on 127.0.0.1 only.

Logs: `journalctl -u quiet-backend -f`. Each topic run is also recorded in the `runs` table.

Update after a push: `cd /opt/quiet && git pull`, rebuild both, then `sudo systemctl restart quiet-backend quiet-web`.

## API

All backend routes need `Authorization: Bearer $APP_TOKEN`.

| Method | Path | |
|---|---|---|
| GET | `/api/today` | today's page JSON |
| POST | `/api/topics` `{text}` | new trial topic, runs it, returns its section (429 over the daily cap) |
| POST | `/api/topics/{id}/follow` | follow daily |
| PATCH | `/api/topics/{id}` `{text}` | adjust rules ("less company news") and re-run |
| DELETE | `/api/topics/{id}` | drop |
| POST | `/api/admin/run-daily` | run the daily job now |
