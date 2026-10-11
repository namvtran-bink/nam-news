# Quiet

A calm, finite news page, ready every morning at 05:30 (Asia/Saigon), plus one bar to follow a new topic on the fly. See `CLAUDE.md` for the product brief and `DECISIONS.md` for choices made while building.

```
backend/   Go: API + scheduler + SQLite (binds 127.0.0.1:8080)
web/       Next.js: login, Today page, proxy routes (binds 127.0.0.1:3000)
deploy/    Caddyfile, systemd units, backup script
```

## Run locally

Needs Go 1.24+, Node 22+, a Tavily key and an Anthropic key.

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

## Deploy to a VPS (Debian/Ubuntu)

```sh
# 1. Packages
sudo apt install -y caddy sqlite3 git golang nodejs npm   # or install Go 1.24+ / Node 22+ from upstream

# 2. User, code, config
sudo useradd --system --home /var/lib/quiet --shell /usr/sbin/nologin quiet
sudo git clone <this repo> /opt/quiet
sudo mkdir -p /etc/quiet /var/lib/quiet && sudo chown quiet:quiet /var/lib/quiet
sudo cp /opt/quiet/.env.example /etc/quiet/quiet.env
sudo chown root:quiet /etc/quiet/quiet.env && sudo chmod 640 /etc/quiet/quiet.env
sudoedit /etc/quiet/quiet.env    # keys, APP_TOKEN, APP_PASSWORD

# 3. Build
cd /opt/quiet/backend && sudo go build -o /opt/quiet/bin/quiet ./cmd/quiet
cd /opt/quiet/web && sudo npm ci && sudo npm run build
sudo chown -R quiet:quiet /opt/quiet/web/.next

# 4. Services
sudo cp /opt/quiet/deploy/quiet-*.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now quiet-backend quiet-web

# 5. TLS: edit the domain in deploy/Caddyfile (your domain, or <ip>.sslip.io), then
sudo cp /opt/quiet/deploy/Caddyfile /etc/caddy/Caddyfile && sudo systemctl reload caddy

# 6. Backups, daily at 04:15, keeps 7
echo '15 4 * * * quiet /opt/quiet/deploy/backup.sh' | sudo tee /etc/cron.d/quiet-backup

# 7. Seed your topics (each runs once and is followed daily)
sudo -u quiet bash -c 'set -a; . /etc/quiet/quiet.env; /opt/quiet/bin/quiet run -follow "AI policy, substantive only"'

# 8. Prove the daily job once
sudo -u quiet bash -c 'set -a; . /etc/quiet/quiet.env; curl -s -X POST -H "Authorization: Bearer $APP_TOKEN" localhost:8080/api/admin/run-daily'
```

Only ports 80/443 need to be open (`sudo ufw allow 22,80,443/tcp && sudo ufw enable`); the backend and Next.js listen on 127.0.0.1 only.

Logs: `journalctl -u quiet-backend -f`. Each topic run is also recorded in the `runs` table.

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
