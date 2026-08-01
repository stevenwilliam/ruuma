# Production Deployment Handbook — ruuma

**Version:** 1.0
**Date:** 2 August 2026
**Target:** a **completely empty Ubuntu 24.04 / 26.04 server**, single node.
**Domain:** `ruuma.id` (customer) and `admin.ruuma.id` (admin).

Every command is copy-paste and uses **full absolute paths** — a pasted command
can never run in the wrong directory. The editor is `vi` throughout.

Run everything as `root` unless a step says otherwise.

> **Read first.** Two decisions shape this deployment. Payment is **manual bank
> transfer** verified by a human (D13/D26), so finance staff need accounts on
> day one. WhatsApp goes through **WAHA**, an unofficial gateway whose ban risk
> the owner accepted (D11) — §12 covers what to do when the session drops.

---

## 0. What you need before you start

| Thing | Why |
|---|---|
| A server with a public IPv4 address | nginx terminates TLS on 80/443 |
| DNS `A` records for `ruuma.id`, `www.ruuma.id`, `admin.ruuma.id` → that IP | certbot verifies over HTTP |
| An SMTP account (host, user, password, from-address) | email verification (docs/00 Q9) |
| A WhatsApp number you can scan a QR code with | WAHA session |
| The group's real store data | addresses, phones, bank accounts |

---

## 1. Base system

```bash
apt-get update && apt-get -y upgrade
export DEBIAN_FRONTEND=noninteractive
apt-get -y install \
  ca-certificates curl gnupg lsb-release ufw git vim jq unzip \
  build-essential nginx software-properties-common
```

```bash
timedatectl set-timezone Asia/Jakarta
timedatectl
```

The server clock is Jakarta so that logs read naturally to the operators.
The application still stores every timestamp in UTC and converts explicitly
(BR-1.3.1/2) — it does not depend on this setting.

### 1.1 Firewall

```bash
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable
ufw status verbose
```

Nothing else is exposed. PostgreSQL, MinIO, the API and `/metrics` all bind to
`127.0.0.1` (docs/12, A05).

### 1.2 Service user

```bash
adduser --system --group --home /opt/ruuma --shell /usr/sbin/nologin ruuma
install -d -o ruuma -g ruuma -m 0755 /opt/ruuma /opt/ruuma/bin /opt/ruuma/web
install -d -o root  -g ruuma -m 0750 /etc/ruuma
install -d -o ruuma -g ruuma -m 0750 /var/backups/ruuma /var/backups/ruuma/db
```

---

## 2. PostgreSQL 18

```bash
install -d /usr/share/postgresql-common/pgdg
curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc \
  -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc
echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] \
https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" \
  > /etc/apt/sources.list.d/pgdg.list
apt-get update
apt-get -y install postgresql-18 postgresql-client-18
systemctl enable --now postgresql
systemctl status postgresql --no-pager
```

### 2.1 Roles and database

Two roles, deliberately: migrations run as the **owner**, the application runs
as a **least-privilege** role that cannot create anything (docs/09 §5).

```bash
OWNER_PW="$(openssl rand -hex 24)"
APP_PW="$(openssl rand -hex 24)"
echo "owner password: ${OWNER_PW}"
echo "app password:   ${APP_PW}"
```

Write those two down now — they go into `/etc/ruuma/ruuma.env` in §6.

```bash
sudo -u postgres psql -v ON_ERROR_STOP=1 <<SQL
CREATE ROLE ruuma_owner LOGIN PASSWORD '${OWNER_PW}';
CREATE ROLE ruuma_app   LOGIN PASSWORD '${APP_PW}';
CREATE DATABASE ruuma OWNER ruuma_owner;
SQL
```

```bash
sudo -u postgres psql -v ON_ERROR_STOP=1 -d ruuma <<'SQL'
-- The app may use the schema but never create in it.
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO ruuma_app;
ALTER DEFAULT PRIVILEGES FOR ROLE ruuma_owner IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO ruuma_app;
ALTER DEFAULT PRIVILEGES FOR ROLE ruuma_owner IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO ruuma_app;
SQL
```

### 2.2 Listen on localhost only

```bash
vi /etc/postgresql/18/main/postgresql.conf
```

Confirm:

```
listen_addresses = 'localhost'
```

```bash
systemctl restart postgresql
ss -lntp | grep 5432
```

The socket must show `127.0.0.1:5432`, never `0.0.0.0`.

---

## 3. Object storage — MinIO

Payment proofs are financial evidence. The bucket stays **private** and objects
are reachable only through short-lived presigned URLs (BR-2.6.11).

```bash
curl -fsSL https://dl.min.io/server/minio/release/linux-amd64/minio \
  -o /usr/local/bin/minio
chmod +x /usr/local/bin/minio
adduser --system --group --home /var/lib/minio --shell /usr/sbin/nologin minio
install -d -o minio -g minio -m 0750 /var/lib/minio
```

```bash
MINIO_USER="ruuma-$(openssl rand -hex 4)"
MINIO_PW="$(openssl rand -hex 24)"
echo "minio access key: ${MINIO_USER}"
echo "minio secret key: ${MINIO_PW}"

cat > /etc/default/minio <<EOF
MINIO_ROOT_USER=${MINIO_USER}
MINIO_ROOT_PASSWORD=${MINIO_PW}
MINIO_VOLUMES=/var/lib/minio
MINIO_OPTS=--address 127.0.0.1:9000 --console-address 127.0.0.1:9001
EOF
chmod 0640 /etc/default/minio
```

```bash
cat > /etc/systemd/system/minio.service <<'EOF'
[Unit]
Description=MinIO object storage for ruuma
After=network-online.target
Wants=network-online.target

[Service]
User=minio
Group=minio
EnvironmentFile=/etc/default/minio
ExecStart=/usr/local/bin/minio server $MINIO_OPTS $MINIO_VOLUMES
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/var/lib/minio

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now minio
systemctl status minio --no-pager
```

MinIO listens on loopback only; nginx never proxies it.

---

## 4. Build the application

Go and Node are needed to build, not to run. The result is a single static
binary plus a folder of static files.

```bash
cd /usr/local
curl -fsSLO https://go.dev/dl/go1.26.5.linux-amd64.tar.gz
rm -rf /usr/local/go && tar -C /usr/local -xzf /usr/local/go1.26.5.linux-amd64.tar.gz
rm -f /usr/local/go1.26.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
export PATH=$PATH:/usr/local/go/bin
go version
```

```bash
curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
apt-get -y install nodejs
node --version && npm --version
```

```bash
install -d -o root -g root -m 0755 /opt/ruuma/src
git clone https://github.com/stevenwilliam/ruuma.git /opt/ruuma/src
cd /opt/ruuma/src
git checkout main
```

```bash
cd /opt/ruuma/src
export PATH=$PATH:/usr/local/go/bin
go build -trimpath \
  -ldflags "-s -w -X main.version=$(git describe --tags --always) -X main.commit=$(git rev-parse --short HEAD)" \
  -o /opt/ruuma/bin/ruuma ./cmd/api
chown ruuma:ruuma /opt/ruuma/bin/ruuma
/opt/ruuma/bin/ruuma version
```

```bash
cd /opt/ruuma/src/web
npm ci
npm run build
rm -rf /opt/ruuma/web && cp -r /opt/ruuma/src/web/dist /opt/ruuma/web
chown -R ruuma:ruuma /opt/ruuma/web
```

---

## 5. WhatsApp gateway (WAHA)

```bash
apt-get -y install docker.io
systemctl enable --now docker

WAHA_KEY="$(openssl rand -hex 16)"
echo "waha api key: ${WAHA_KEY}"

docker run -d --name waha --restart unless-stopped \
  -p 127.0.0.1:3000:3000 \
  -v waha_sessions:/app/.sessions \
  -e WAHA_API_KEY="${WAHA_KEY}" \
  -e WHATSAPP_DEFAULT_ENGINE=NOWEB \
  devlikeapro/waha
```

Two things that will bite otherwise:

- **Pin `WAHA_API_KEY`.** Without it WAHA generates a new key on every restart
  and ruuma's sends start failing silently into the retry queue.
- **Persist `/app/.sessions`.** Without the volume every restart needs a fresh
  QR scan.

Create the session named exactly `default` (that is what `WAHA_SESSION` expects)
and link the phone by scanning its QR code from a machine that can reach the
server:

```bash
ssh -L 3000:127.0.0.1:3000 root@YOUR_SERVER_IP
# then open http://127.0.0.1:3000 in your own browser and scan
```

---

## 6. Configuration

```bash
vi /etc/ruuma/ruuma.env
```

```ini
APP_ENV=production
APP_PORT=8080
APP_BASE_URL=https://ruuma.id
APP_ADMIN_BASE_URL=https://admin.ruuma.id
LOG_LEVEL=info
TRUSTED_PROXIES=127.0.0.1
CORS_ALLOWED_ORIGINS=https://ruuma.id,https://www.ruuma.id,https://admin.ruuma.id

# The application connects as the least-privilege role.
DATABASE_URL=postgres://ruuma_app:APP_PASSWORD_FROM_STEP_2@127.0.0.1:5432/ruuma?sslmode=disable
DATABASE_MAX_OPEN_CONNS=25
DATABASE_MAX_IDLE_CONNS=5

# openssl rand -base64 48
JWT_SIGNING_KEY=PASTE_A_FRESH_48_BYTE_KEY
JWT_PREVIOUS_KEY=
JWT_ISSUER=ruuma

MINIO_ENDPOINT=127.0.0.1:9000
MINIO_ACCESS_KEY=MINIO_ACCESS_FROM_STEP_3
MINIO_SECRET_KEY=MINIO_SECRET_FROM_STEP_3
MINIO_BUCKET=ruuma
MINIO_USE_SSL=false

SMTP_HOST=smtp.your-provider.example
SMTP_PORT=587
SMTP_USERNAME=no-reply@ruuma.id
SMTP_PASSWORD=YOUR_SMTP_PASSWORD
SMTP_FROM_EMAIL=no-reply@ruuma.id
SMTP_FROM_NAME=Ruuma Eatery
SMTP_TLS=true

WAHA_URL=http://127.0.0.1:3000
WAHA_SESSION=default
WAHA_API_KEY=WAHA_KEY_FROM_STEP_5

# Filled in when the OAuth apps exist (docs/00 Q8); the providers stay disabled
# until then and email + phone sign-in cover launch.
GOOGLE_OAUTH_CLIENT_ID=
GOOGLE_OAUTH_CLIENT_SECRET=
GOOGLE_OAUTH_REDIRECT_URL=https://ruuma.id/api/v1/auth/oauth/google/callback
INSTAGRAM_OAUTH_CLIENT_ID=
INSTAGRAM_OAUTH_CLIENT_SECRET=
INSTAGRAM_OAUTH_REDIRECT_URL=https://ruuma.id/api/v1/auth/oauth/instagram/callback
```

```bash
chown root:ruuma /etc/ruuma/ruuma.env
chmod 0640 /etc/ruuma/ruuma.env
```

Business settings — tax rate, slot length, capacity, cutoffs, message templates —
are **not** in this file. They live in `sys_parameters` and are edited in the
admin UI without a deploy (BR-1.4.1).

---

## 7. Migrate and bootstrap

Migrations run as the **owner** role, not the application role:

```bash
cd /opt/ruuma/src
DATABASE_URL="postgres://ruuma_owner:OWNER_PASSWORD_FROM_STEP_2@127.0.0.1:5432/ruuma?sslmode=disable" \
JWT_SIGNING_KEY="$(grep '^JWT_SIGNING_KEY=' /etc/ruuma/ruuma.env | cut -d= -f2-)" \
/opt/ruuma/bin/ruuma migrate
```

```bash
DATABASE_URL="postgres://ruuma_owner:OWNER_PASSWORD_FROM_STEP_2@127.0.0.1:5432/ruuma?sslmode=disable" \
JWT_SIGNING_KEY="$(grep '^JWT_SIGNING_KEY=' /etc/ruuma/ruuma.env | cut -d= -f2-)" \
/opt/ruuma/bin/ruuma migrate --status
```

### 7.1 The first owner account

ruuma ships with **no default credentials** (docs/12, A05). Create the first
owner explicitly:

```bash
cd /opt/ruuma/src
set -a; . /etc/ruuma/ruuma.env; set +a
/opt/ruuma/bin/ruuma create-owner --email owner@ruuma.id --name "Owner"
```

The generated password is printed **once**. Save it in your password manager,
sign in at `https://admin.ruuma.id`, and change it immediately — the account is
flagged `must_change_password`.

Running the command again is refused while an active owner exists.

> **Never run `ruuma seed` here.** It loads demo stores and refuses to run when
> `APP_ENV=production`, but the habit is the danger.

---

## 8. systemd units

```bash
cat > /etc/systemd/system/ruuma-api.service <<'EOF'
[Unit]
Description=ruuma API
After=network-online.target postgresql.service minio.service
Wants=network-online.target

[Service]
User=ruuma
Group=ruuma
EnvironmentFile=/etc/ruuma/ruuma.env
WorkingDirectory=/opt/ruuma
ExecStart=/opt/ruuma/bin/ruuma serve
Restart=always
RestartSec=5
# Graceful shutdown drains in-flight requests (docs/05 §7).
KillSignal=SIGTERM
TimeoutStopSec=30
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/ruuma
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
```

```bash
cat > /etc/systemd/system/ruuma-worker.service <<'EOF'
[Unit]
Description=ruuma background worker (slots, notifications, cleanup)
After=network-online.target postgresql.service ruuma-api.service
Wants=network-online.target

[Service]
User=ruuma
Group=ruuma
EnvironmentFile=/etc/ruuma/ruuma.env
WorkingDirectory=/opt/ruuma
ExecStart=/opt/ruuma/bin/ruuma worker
Restart=always
RestartSec=10
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/ruuma

[Install]
WantedBy=multi-user.target
EOF
```

```bash
systemctl daemon-reload
systemctl enable --now ruuma-api ruuma-worker
systemctl status ruuma-api --no-pager
curl -s http://127.0.0.1:8080/health
```

The worker materialises slots and dispatches notifications. It **never cancels
an order** — phase 1 has no auto-cancel (D25).

---

## 9. nginx and TLS

```bash
cat > /etc/nginx/sites-available/ruuma <<'EOF'
# Customer site
server {
    listen 80;
    server_name ruuma.id www.ruuma.id;

    root /opt/ruuma/web;
    index index.html;

    client_max_body_size 8m;   # payment proofs and menu photos

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 60s;
    }

    location /health { proxy_pass http://127.0.0.1:8080/health; }

    # Single-page app: unknown paths render the app, not a 404.
    location / { try_files $uri $uri/ /index.html; }

    location ~* \.(js|css|png|jpg|jpeg|webp|svg|woff2)$ {
        expires 30d;
        add_header Cache-Control "public, immutable";
    }
}

# Admin site — a separate host, matching the separate router group in the API.
server {
    listen 80;
    server_name admin.ruuma.id;

    root /opt/ruuma/web;
    index index.html;
    client_max_body_size 10m;

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location / { try_files $uri $uri/ /index.html; }
}
EOF

ln -sf /etc/nginx/sites-available/ruuma /etc/nginx/sites-enabled/ruuma
rm -f /etc/nginx/sites-enabled/default
nginx -t && systemctl reload nginx
```

`/metrics` is deliberately absent: it is bound to `127.0.0.1:9090` and must
never be proxied (docs/12, A05).

### 9.1 Certificates

```bash
apt-get -y install certbot python3-certbot-nginx
certbot --nginx -d ruuma.id -d www.ruuma.id -d admin.ruuma.id \
  --agree-tos -m owner@ruuma.id --redirect --no-eff-email
systemctl status certbot.timer --no-pager
```

### 9.2 HSTS

Only after you are certain every subdomain will stay on HTTPS — this is hard to
undo:

```bash
vi /etc/nginx/sites-available/ruuma
```

Add inside both `server` blocks that listen on 443:

```
add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload" always;
```

```bash
nginx -t && systemctl reload nginx
```

---

## 10. First-run checks

```bash
curl -s https://ruuma.id/health | jq .
curl -s https://ruuma.id/api/v1/stores | jq .
curl -sI https://ruuma.id | grep -iE "strict-transport|content-security|x-frame|x-content"
curl -s -o /dev/null -w "%{http_code}\n" https://ruuma.id/api/v1/admin/sys-parameters   # expect 401
curl -s -o /dev/null -w "%{http_code}\n" https://ruuma.id/metrics                        # expect 404
```

Then, in the admin UI at `https://admin.ruuma.id`:

1. Change the owner password.
2. Create the stores (§0 data): code, name, address, phone, fulfilment modes.
3. Set **opening hours per weekday per mode**, marking closed days closed.
4. Add each store's **bank account** and mark one primary — checkout cannot
   complete without it.
5. Create the menu, then set per-store availability and price overrides.
6. Create staff accounts and assign each to its store(s).
7. Review **Parameters**: tax rate, slot length, capacity, lead time, cutoff,
   max advance days, unpaid-order cap.
8. Activate the stores.

Slots appear once a store is active and has hours; the worker materialises them
within ten minutes, or immediately on restart.

---

## 11. Backups

```bash
cat > /usr/local/bin/ruuma-backup <<'EOF'
#!/usr/bin/env bash
# Nightly backup: database, object storage and configuration.
set -euo pipefail

STAMP="$(date +%Y%m%d-%H%M%S)"
DB_DIR=/var/backups/ruuma/db
OBJ_DIR=/var/backups/ruuma/objects

mkdir -p "${DB_DIR}" "${OBJ_DIR}"

# Payment proofs are financial evidence: the database alone is not a backup.
sudo -u postgres pg_dump -Fc ruuma > "${DB_DIR}/ruuma-${STAMP}.dump"
tar -czf "${OBJ_DIR}/objects-${STAMP}.tar.gz" -C /var/lib/minio .

find "${DB_DIR}"  -name 'ruuma-*.dump'      -mtime +30 -delete
find "${OBJ_DIR}" -name 'objects-*.tar.gz'  -mtime +30 -delete

echo "backup complete: ${STAMP}"
EOF
chmod +x /usr/local/bin/ruuma-backup
```

```bash
cat > /etc/systemd/system/ruuma-backup.service <<'EOF'
[Unit]
Description=ruuma nightly backup
[Service]
Type=oneshot
ExecStart=/usr/local/bin/ruuma-backup
EOF

cat > /etc/systemd/system/ruuma-backup.timer <<'EOF'
[Unit]
Description=Run the ruuma backup nightly
[Timer]
OnCalendar=*-*-* 02:30:00
Persistent=true
[Install]
WantedBy=timers.target
EOF

systemctl daemon-reload
systemctl enable --now ruuma-backup.timer
systemctl list-timers ruuma-backup --no-pager
```

**Copy the backups off this machine.** A backup on the server that dies with the
server is not a backup. Add an `rclone`/`rsync` step to a second location, and
back up `/etc/ruuma/ruuma.env` separately, encrypted.

### 11.1 Restore drill — do this quarterly

```bash
sudo -u postgres createdb ruuma_restore_test
sudo -u postgres pg_restore -d ruuma_restore_test \
  /var/backups/ruuma/db/$(ls -t /var/backups/ruuma/db | head -1)
sudo -u postgres psql -d ruuma_restore_test -c "select count(*) from orders;"
sudo -u postgres dropdb ruuma_restore_test
```

Time it. The target is a full restore inside 30 minutes (docs/09 §6).

---

## 12. Operating notes

### 12.1 Deploying a new version

```bash
cd /opt/ruuma/src
git pull
export PATH=$PATH:/usr/local/go/bin

cp /opt/ruuma/bin/ruuma /opt/ruuma/bin/ruuma.previous
rm -rf /opt/ruuma/web.previous && cp -r /opt/ruuma/web /opt/ruuma/web.previous

go build -trimpath \
  -ldflags "-s -w -X main.version=$(git describe --tags --always) -X main.commit=$(git rev-parse --short HEAD)" \
  -o /opt/ruuma/bin/ruuma ./cmd/api

cd /opt/ruuma/src/web && npm ci && npm run build
rm -rf /opt/ruuma/web && cp -r /opt/ruuma/src/web/dist /opt/ruuma/web
chown -R ruuma:ruuma /opt/ruuma/web /opt/ruuma/bin/ruuma

# Migrations run before the new binary serves.
DATABASE_URL="postgres://ruuma_owner:OWNER_PASSWORD@127.0.0.1:5432/ruuma?sslmode=disable" \
JWT_SIGNING_KEY="$(grep '^JWT_SIGNING_KEY=' /etc/ruuma/ruuma.env | cut -d= -f2-)" \
/opt/ruuma/bin/ruuma migrate

systemctl restart ruuma-api ruuma-worker
curl -s http://127.0.0.1:8080/health | jq .
```

### 12.2 Rolling back

```bash
cp /opt/ruuma/bin/ruuma.previous /opt/ruuma/bin/ruuma
rm -rf /opt/ruuma/web && cp -r /opt/ruuma/web.previous /opt/ruuma/web
systemctl restart ruuma-api ruuma-worker
```

Roll the binary back first. Only if the migration is the problem, apply its
`.down.sql` — every migration has one and CI runs up → down → up.

### 12.3 WhatsApp session dropped

Symptoms: notifications stack up in the `notifications` table with `status =
'failed'`. WAHA is an unofficial gateway (D11) and its session can drop.

```bash
docker logs --tail 50 waha
ssh -L 3000:127.0.0.1:3000 root@YOUR_SERVER_IP   # then re-scan the QR code
```

While it is down, ruuma keeps retrying with backoff and the orders themselves
are unaffected — customers still see everything on their order page. If the
number is banned, switch `notify.provider` in **Parameters** to `meta_cloud`
(with credentials) or `log`, with no deploy.

### 12.4 Rotating the JWT signing key

```bash
vi /etc/ruuma/ruuma.env
# JWT_PREVIOUS_KEY=<the current key>
# JWT_SIGNING_KEY=<a new one: openssl rand -base64 48>
systemctl restart ruuma-api
```

Leave `JWT_PREVIOUS_KEY` in place for one refresh-token lifetime
(`auth.refresh_token_days`, default 30), then clear it and restart again.

### 12.5 Logs

```bash
journalctl -u ruuma-api -f
journalctl -u ruuma-worker -f
journalctl -u ruuma-api --since "1 hour ago" | jq -R 'fromjson? // .'
```

Logs are structured JSON with a request id, and phone numbers, emails and
addresses are redacted at write time (docs/12, A09).

---

## 13. Go-live checklist

- [ ] `ufw status` shows only OpenSSH, 80 and 443
- [ ] `ss -lntp` shows PostgreSQL, MinIO, the API and `:9090` on `127.0.0.1` only
- [ ] `https://ruuma.id/health` returns the expected version and commit
- [ ] `https://ruuma.id/metrics` returns 404
- [ ] Security headers present, including HSTS once §9.2 is done
- [ ] `/etc/ruuma/ruuma.env` is `0640 root:ruuma`
- [ ] The owner password has been changed from the bootstrap value
- [ ] Every store has hours, a primary bank account and correct closed days
- [ ] Finance staff exist and can see their queue
- [ ] A test order end to end: order → transfer → proof → verify → WhatsApp →
      kitchen board → handover
- [ ] `systemctl list-timers` shows the backup timer, and one backup exists
- [ ] A restore drill has been run and timed
- [ ] Backups are copied off the machine
- [ ] The WAHA number is linked and a test message arrives
