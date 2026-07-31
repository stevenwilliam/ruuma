# 13a — Development Server Preparation (`claudedev`, fresh install, copy‑paste)

**Version:** 2.0
**Date:** 31 July 2026
**Target OS:** Ubuntu 26.04 LTS (fresh install)
**Server name:** `claudedev` — a **shared development server for many projects**,
not ruuma-only. ruuma is the first project onboarded onto it and is used as the
worked example throughout.

> **Conventions in this handbook**
> - Every path is **absolute** (full path), never relative, to avoid running a
>   command in the wrong directory.
> - The default editor is **`vi`**. All "edit this file" steps use `vi`.
> - The Linux user shared across projects is **`dev`**, home **`/home/dev`**.
> - Each project lives at **`/home/dev/projects/<project>`** (e.g.
>   `/home/dev/projects/ruuma`). Server-wide config lives under
>   **`/etc/claudedev/`**; per-project config under **`/etc/<project>/`**.
> - `# (as root)` blocks run as root; `# (as dev)` as the `dev` user.
> - Placeholders in `<ANGLE_BRACKETS>` and `<project>` must be replaced.

This handbook has two parts:
- **Part A — Server setup (run once):** everything shared by all projects.
- **Part B — Onboard a project (repeat per project):** the per-project steps.

---

# PART A — Server setup (run once)

## A0. First login (as root)

```bash
# (as root) — over SSH to the fresh box
whoami                     # should print: root
cat /etc/os-release        # confirm: Ubuntu 26.04
```

## A1. System update & base configuration (as root)

```bash
# (as root)
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get -y upgrade
apt-get -y dist-upgrade
apt-get -y autoremove

# Timezone + locale (adjust timezone to your operation)
timedatectl set-timezone Asia/Jakarta
apt-get -y install locales
locale-gen en_US.UTF-8
update-locale LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8

# Hostname — this is the shared dev server
hostnamectl set-hostname claudedev
```

## A2. Essential apt packages (as root)

```bash
# (as root)
apt-get -y install \
  build-essential git curl wget ca-certificates gnupg lsb-release \
  vim unzip zip htop net-tools ufw fail2ban jq make pkg-config \
  software-properties-common apt-transport-https openssh-server nginx
```

## A3. Set `vi` as the system default editor (as root)

```bash
# (as root)
update-alternatives --set editor /usr/bin/vim.basic
update-alternatives --set vi     /usr/bin/vim.basic 2>/dev/null || true

cat >/etc/profile.d/editor.sh <<'EOF'
export EDITOR=vi
export VISUAL=vi
EOF
chmod 0644 /etc/profile.d/editor.sh
```

## A4. Create the shared `dev` user (as root)

```bash
# (as root)
adduser --gecos "" --disabled-password dev
usermod -aG sudo dev
# Nginx runs as www-data; let dev manage project web roots later if needed
```

## A5. Sudoers — `dev` runs sudo without a password prompt (as root)

> Convenient on a controlled dev box. Do **not** copy to production. Always edit
> via a `.d` file and validate with `visudo -c`.

```bash
# (as root)
cat >/etc/sudoers.d/90-dev-nopasswd <<'EOF'
dev ALL=(ALL) NOPASSWD:ALL
EOF
chmod 0440 /etc/sudoers.d/90-dev-nopasswd
visudo -c        # MUST print "parsed OK" before you log out
```

Test in a new SSH session:

```bash
# (as dev)
sudo whoami        # should print: root  (no password prompt)
```

## A6. SSH hardening + keys for `dev` (as root, then edit)

```bash
# (as root)
mkdir -p /home/dev/.ssh
chmod 700 /home/dev/.ssh
touch /home/dev/.ssh/authorized_keys
chmod 600 /home/dev/.ssh/authorized_keys
chown -R dev:dev /home/dev/.ssh

vi /home/dev/.ssh/authorized_keys     # paste your laptop's id_ed25519.pub

vi /etc/ssh/sshd_config
#   PermitRootLogin no
#   PasswordAuthentication no
#   PubkeyAuthentication yes
systemctl restart ssh
```

## A7. Firewall & fail2ban (as root)

```bash
# (as root)
ufw default deny incoming
ufw default allow outgoing
ufw allow OpenSSH
ufw allow 'Nginx Full'          # 80/443 — reverse proxy fronts project ports
ufw --force enable
ufw status verbose

systemctl enable --now fail2ban
```

> Per-project app ports (e.g. 8080, 8081, …) are **not** opened directly — nginx
> proxies them (see A12). Open a raw port only if a project needs it.

## A8. Install Go (latest) (as root)

```bash
# (as root)
cd /usr/local/src
GO_VERSION="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -1)"
echo "Installing ${GO_VERSION}"
curl -fLO "https://go.dev/dl/${GO_VERSION}.linux-amd64.tar.gz"
rm -rf /usr/local/go
tar -C /usr/local -xzf "/usr/local/src/${GO_VERSION}.linux-amd64.tar.gz"

cat >/etc/profile.d/go.sh <<'EOF'
export PATH=$PATH:/usr/local/go/bin:/home/dev/go/bin
export GOPATH=/home/dev/go
EOF
chmod 0644 /etc/profile.d/go.sh
```

Verify (new shell as `dev`):

```bash
# (as dev)
source /etc/profile.d/go.sh
/usr/local/go/bin/go version
mkdir -p /home/dev/go/bin
```

## A9. Shared Go tooling (as dev)

`gin`/`gorm` (and any other libraries) are per-project `go get` dependencies, not
OS installs. Install the shared developer tooling once:

```bash
# (as dev)
source /etc/profile.d/go.sh
go install github.com/air-verse/air@latest              # live reload
go install github.com/go-delve/delve/cmd/dlv@latest     # debugger
go install golang.org/x/tools/gopls@latest              # language server
go install honnef.co/go/tools/cmd/staticcheck@latest    # linter
```

## A10. Install PostgreSQL 18 — one shared instance (as root)

One PostgreSQL server hosts one **role + database per project** (created in
Part B).

```bash
# (as root)
install -d /usr/share/postgresql-common/pgdg
curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc \
  -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc

echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] \
https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" \
  > /etc/apt/sources.list.d/pgdg.list

apt-get update
apt-get -y install postgresql-18 postgresql-client-18
systemctl enable --now postgresql
```

Config file locations (full paths):

- Server config: `/etc/postgresql/18/main/postgresql.conf`
- Client auth:   `/etc/postgresql/18/main/pg_hba.conf`
- Data dir:      `/var/lib/postgresql/18/main`
- Logs:          `/var/log/postgresql/postgresql-18-main.log`

```bash
# (as root) — dev-server defaults: localhost only, scram auth
vi /etc/postgresql/18/main/postgresql.conf
#   listen_addresses = 'localhost'
#   password_encryption = scram-sha-256
systemctl restart postgresql
```

## A11. Global git config (as dev)

```bash
# (as dev)
git config --global user.name  "stevenwilliam"
git config --global user.email "itdept.sfg@gmail.com"
git config --global init.defaultBranch main
git config --global pull.rebase true
git config --global core.editor vi
git config --global core.autocrlf input
git config --global push.autoSetupRemote true
```

### A11.1 Never re-type your password — SSH key (recommended)

```bash
# (as dev)
ssh-keygen -t ed25519 -C "dev@claudedev" -f /home/dev/.ssh/id_ed25519 -N ""
eval "$(ssh-agent -s)"
ssh-add /home/dev/.ssh/id_ed25519
cat /home/dev/.ssh/id_ed25519.pub     # add at https://github.com/settings/keys

# Auto-start the agent for every login shell
cat >>/home/dev/.bashrc <<'EOF'

# Auto-start ssh-agent and load the GitHub key
if [ -z "$SSH_AUTH_SOCK" ]; then
  eval "$(ssh-agent -s)" >/dev/null
  ssh-add /home/dev/.ssh/id_ed25519 2>/dev/null
fi
EOF
ssh -T git@github.com || true
```

### A11.2 Alternative — HTTPS + PAT cached forever

```bash
# (as dev)
git config --global credential.helper 'store'
# First HTTPS push prompts once (username + PAT from
# https://github.com/settings/tokens); saved to /home/dev/.git-credentials.
```

## A12. Reverse proxy for multiple projects (as root)

nginx fronts each project's local port on a subdomain/path, so only 80/443 are
public. Example per-project vhost is created during onboarding (B5).

```bash
# (as root)
mkdir -p /etc/nginx/sites-available /etc/nginx/sites-enabled
systemctl enable --now nginx
```

## A13. Shared WhatsApp notify via WAHA (as root)

Notifications use **WAHA (WhatsApp HTTP API, Core edition)** — free, self-hosted,
and driven by your **own** WhatsApp number (scan a QR once, like WhatsApp Web).
One WAHA container serves every project and CI runner; the shared `dev-notify`
helper just POSTs to it over localhost.

> ⚠️ WAHA is an **unofficial** gateway (it automates WhatsApp Web). Use a
> **secondary / dev WhatsApp number**, not your primary personal one, and keep
> volume low. For customer-facing production messaging use the official Meta
> Cloud API instead (see A13.4).

### A13.1 Install Docker (as root)

```bash
# (as root)
curl -fsSL https://get.docker.com | sh
systemctl enable --now docker
usermod -aG docker dev        # let dev run docker without sudo (re-login to apply)
```

### A13.2 Run WAHA and link your number (as root, then tunnel from laptop)

```bash
# (as root) — bind to localhost only; nothing WhatsApp is exposed publicly.
# Pin ALL three secrets so they survive restarts (replace the values):
#   - dashboard login (WAHA_DASHBOARD_*)
#   - API key (WAHA_API_KEY) — generate once: openssl rand -hex 16
API_KEY="$(openssl rand -hex 16)"; echo "WAHA_API_KEY=$API_KEY"   # note this down
docker run -d --name waha --restart unless-stopped \
  -p 127.0.0.1:3000:3000 \
  -e WAHA_DASHBOARD_USERNAME=admin \
  -e WAHA_DASHBOARD_PASSWORD=change-me \
  -e WAHA_API_KEY="$API_KEY" \
  -e WHATSAPP_DEFAULT_ENGINE=NOWEB \
  devlikeapro/waha
```

> **Use the `NOWEB` engine on a server.** WAHA's default `WEBJS` engine drives a
> headless Chromium, which is heavy and crashes on low-RAM boxes (symptom:
> `500 ... Page.captureScreenshot: Session closed`). `NOWEB` talks the WhatsApp
> protocol directly — no browser — so it's lighter and far more stable for
> notifications. Set `WHATSAPP_DEFAULT_ENGINE=NOWEB` as above.

> **Three separate logins on port 3000 — don't confuse them:**
> - **Dashboard** (the web UI you open) → `WAHA_DASHBOARD_USERNAME` /
>   `WAHA_DASHBOARD_PASSWORD`. Separate from the Linux `dev` account.
> - **Swagger docs** → `WHATSAPP_SWAGGER_USERNAME` / `WHATSAPP_SWAGGER_PASSWORD`.
> - **API calls** (what `dev-notify` uses) → the `WAHA_API_KEY` sent as an
>   `X-Api-Key` header.
>
> ⚠️ WAHA Core **auto-generates `WAHA_API_KEY` on every start** if you don't pin
> it — which would silently break `dev-notify` after any restart. Always pass
> `-e WAHA_API_KEY=...` with a fixed value (as above). To change any secret
> later: `docker rm -f waha` and re-run with new values.

Link the dev WhatsApp number by scanning a QR:

```bash
# (on your LAPTOP) — tunnel the dashboard to your machine
ssh -L 3000:127.0.0.1:3000 dev@<CLAUDEDEV_IP>
# then browse to http://localhost:3000, Start the "default" session,
# and scan the QR with WhatsApp on the dev phone.
```

Create a session named **exactly `default`** (so it matches `WAHA_SESSION` in
`dev-notify` — a mismatched name is a common cause of `422` on send):

```bash
# (as root)
KEY=<YOUR_WAHA_API_KEY>
curl -s -X POST http://127.0.0.1:3000/api/sessions \
  -H "Content-Type: application/json" -H "X-Api-Key: $KEY" \
  -d '{"name":"default","start":true}'; echo
# verify: status should progress to WORKING after you scan the QR
curl -s -H "X-Api-Key: $KEY" http://127.0.0.1:3000/api/sessions; echo
```

> If the dashboard shows **"Server connection failed / set right API key"**, open
> its **Configuration** (gear icon) and set URL `http://localhost:3000` and the
> **API Key** = your pinned `WAHA_API_KEY`. The dashboard stores this in the
> browser and can't reach the API without it, even after you log in. Verify the
> key from the server with:
> `curl -s -o /dev/null -w "%{http_code}\n" -H "X-Api-Key: $KEY" http://127.0.0.1:3000/api/sessions`
> (expect `200`).

### A13.3 Config + the `dev-notify` helper (as root)

```bash
# (as root)
mkdir -p /etc/claudedev
cat >/etc/claudedev/whatsapp.env <<'EOF'
WAHA_URL=http://127.0.0.1:3000
WAHA_SESSION=default
WHATSAPP_TO=628176315568
WAHA_API_KEY=PASTE_THE_SAME_KEY_YOU_PINNED_IN_A13.2
EOF
chmod 0640 /etc/claudedev/whatsapp.env
chown root:dev /etc/claudedev/whatsapp.env

cat >/usr/local/bin/dev-notify <<'EOF'
#!/usr/bin/env bash
# Usage: dev-notify "<message>"   (prefix the message with [project] yourself)
set -euo pipefail
set -a; source /etc/claudedev/whatsapp.env; set +a
MSG="${*:-claudedev notification}"
curl -fsS -X POST "${WAHA_URL}/api/sendText" \
  -H "Content-Type: application/json" \
  -H "X-Api-Key: ${WAHA_API_KEY}" \
  -d "$(jq -n --arg s "$WAHA_SESSION" --arg to "${WHATSAPP_TO}@c.us" --arg body "$MSG" \
        '{session:$s, chatId:$to, text:$body}')"
EOF
chmod 0755 /usr/local/bin/dev-notify
```

> Note: WAHA addresses a personal number as `<number>@c.us` (the helper appends
> it). No API token / phone-ID / message templates are needed — that's the
> whole point of WAHA over the Meta API.

Test:

```bash
# (as dev)
/usr/local/bin/dev-notify "[claudedev] server is up 🚀"
```

### A13.4 Two-way ChatOps (optional)

Point WAHA's webhook at a project endpoint so inbound messages reach your app:
run WAHA with `-e WHATSAPP_HOOK_URL=http://127.0.0.1:8080/api/v1/webhooks/whatsapp`
(and `-e WHATSAPP_HOOK_EVENTS=message`). The inbound handler is a per-project
code task.

### A13.5 Official alternative (production / customer messaging)

For customer-facing messaging, use the **Meta WhatsApp Cloud API** instead of
WAHA — it can't get your number banned and won't break on WhatsApp updates.
Set `WHATSAPP_TOKEN` / `WHATSAPP_PHONE_ID` in the env and POST to
`https://graph.facebook.com/v20.0/<PHONE_ID>/messages` (Bearer token,
`messaging_product:"whatsapp"`). Requires a verified business number and
approved message templates for business-initiated sends.

## A14. Projects root (as dev)

```bash
# (as dev)
mkdir -p /home/dev/projects
```

---

# PART B — Onboard a project (repeat per project)

Worked example uses `<project> = ruuma`. Substitute the project name throughout.

## B1. Clone the repo (as dev)

```bash
# (as dev)
cd /home/dev/projects
git clone git@github.com:stevenwilliam/ruuma.git            # SSH (A11.1)
# or: git clone https://github.com/stevenwilliam/ruuma.git  # HTTPS (A11.2)
ls -la /home/dev/projects/ruuma
```

## B2. Create the project's PostgreSQL role & database (as root)

```bash
# (as root)
sudo -u postgres psql <<'SQL'
CREATE ROLE ruuma WITH LOGIN PASSWORD '<STRONG_DB_PASSWORD>';
CREATE DATABASE ruuma OWNER ruuma;
GRANT ALL PRIVILEGES ON DATABASE ruuma TO ruuma;
SQL

# Per-project auth line
vi /etc/postgresql/18/main/pg_hba.conf
#   local   ruuma   ruuma                     scram-sha-256
#   host    ruuma   ruuma   127.0.0.1/32      scram-sha-256
systemctl reload postgresql
```

Test:

```bash
# (as dev)
PGPASSWORD='<STRONG_DB_PASSWORD>' psql -h 127.0.0.1 -U ruuma -d ruuma -c '\conninfo'
```

## B3. Per-project config directory (as root)

```bash
# (as root)
mkdir -p /etc/ruuma
cat >/etc/ruuma/ruuma.env <<'EOF'
APP_PORT=8080
DATABASE_URL=postgres://ruuma:<STRONG_DB_PASSWORD>@127.0.0.1:5432/ruuma
# add project-specific settings here; anything business-configurable lives in
# the sys_parameters table, not in this file
EOF
chmod 0640 /etc/ruuma/ruuma.env
chown root:dev /etc/ruuma/ruuma.env
```

## B4. Register a CI/CD runner for this repo (as dev, then root)

```bash
# (as dev)
mkdir -p /home/dev/actions-runner/ruuma
cd /home/dev/actions-runner/ruuma
RUNNER_VER="2.320.0"    # check https://github.com/actions/runner/releases
curl -fLO "https://github.com/actions/runner/releases/download/v${RUNNER_VER}/actions-runner-linux-x64-${RUNNER_VER}.tar.gz"
tar xzf "/home/dev/actions-runner/ruuma/actions-runner-linux-x64-${RUNNER_VER}.tar.gz"
# Token: GitHub repo → Settings → Actions → Runners → New self-hosted runner
./config.sh --url https://github.com/stevenwilliam/ruuma --token <RUNNER_TOKEN> \
  --name claudedev-ruuma --labels claudedev,ruuma --unattended
```

```bash
# (as root)
cd /home/dev/actions-runner/ruuma
./svc.sh install dev
./svc.sh start
./svc.sh status
```

### B4.1 Example workflow — `/home/dev/projects/ruuma/.github/workflows/ci.yml`

```yaml
name: ruuma-ci
on:
  push:
    branches: [ main ]
jobs:
  build-test-deploy:
    runs-on: [self-hosted, ruuma]
    steps:
      - uses: actions/checkout@v4
      - name: Set up Go
        run: echo "/usr/local/go/bin" >> "$GITHUB_PATH"
      - name: Vet & test
        run: |
          go vet ./...
          go test ./...
      - name: Build
        run: go build -o /home/dev/projects/ruuma/bin/ruuma ./cmd/api
      - name: WhatsApp notify (success)
        if: success()
        run: /usr/local/bin/dev-notify "[ruuma] ✅ CI passed ${{ github.sha }}"
      - name: WhatsApp notify (failure)
        if: failure()
        run: /usr/local/bin/dev-notify "[ruuma] ❌ CI FAILED ${{ github.sha }}"
```

## B5. Reverse-proxy vhost for the project (as root)

```bash
# (as root)
cat >/etc/nginx/sites-available/ruuma <<'EOF'
server {
    listen 80;
    server_name ruuma.claudedev.local;   # or a real subdomain
    location / {
        proxy_pass http://127.0.0.1:8080; # APP_PORT from /etc/ruuma/ruuma.env
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
EOF
ln -sf /etc/nginx/sites-available/ruuma /etc/nginx/sites-enabled/ruuma
nginx -t && systemctl reload nginx
```

## B6. Per-project verification (as dev)

```bash
# (as dev)
psql -h 127.0.0.1 -U ruuma -d ruuma -c '\conninfo'          # DB reachable
git -C /home/dev/projects/ruuma remote -v                    # repo cloned
sudo /home/dev/actions-runner/ruuma/svc.sh status            # runner up
/usr/local/bin/dev-notify "[ruuma] onboarding complete ✅"   # notify works
```

To onboard the next project, repeat **B1–B6** with its name.

---

# Server-level verification checklist (as dev)

```bash
# (as dev)
/usr/local/go/bin/go version                                 # Go installed
sudo whoami                                                  # NOPASSWD sudo
ssh -T git@github.com || true                                # git SSH auth
sudo systemctl status postgresql --no-pager                  # PG running
sudo systemctl status nginx --no-pager                       # proxy running
docker ps --filter name=waha                                 # WAHA container up
/usr/local/bin/dev-notify "[claudedev] checklist complete ✅"
echo "EDITOR=$EDITOR"                                        # should be vi
hostnamectl | grep -i hostname                               # claudedev
```

Once a project's code lands, run it with `go run ./cmd/api serve` from
`/home/dev/projects/<project>` (see that project's `docs/RUN-WHEN-BACK.md`).
