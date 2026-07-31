# 13a — Development Server Preparation (fresh install, copy‑paste)

**Version:** 1.0
**Date:** 31 July 2026
**Target OS:** Ubuntu 26.04 LTS (fresh install)
**Purpose:** turn an empty Ubuntu box into a ready ruuma development server — Go
(gin + gorm), PostgreSQL 18, git without password prompts, CI/CD (self‑hosted
GitHub Actions runner), and WhatsApp notify/interaction.

> **Conventions in this handbook**
> - Every path is **absolute** (full path), never relative, to avoid running a
>   command in the wrong directory.
> - The default editor is **`vi`**. All "edit this file" steps use `vi`.
> - The development Linux user is **`dev`** with home **`/home/dev`**. Replace
>   `dev` everywhere if you choose another name.
> - Lines beginning with `#` inside a code block are comments; the rest is
>   copy‑paste. Blocks prefixed `# (as root)` run as root; `# (as dev)` run as
>   the `dev` user.
> - Placeholders in `<ANGLE_BRACKETS>` must be replaced before running.

---

## 0. First login (as root)

```bash
# (as root) — over SSH to the fresh box
whoami                     # should print: root
cat /etc/os-release        # confirm: Ubuntu 26.04
```

---

## 1. System update & base configuration (as root)

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

# Hostname
hostnamectl set-hostname ruuma-dev
```

---

## 2. Essential apt packages (as root)

```bash
# (as root)
apt-get -y install \
  build-essential \
  git \
  curl \
  wget \
  ca-certificates \
  gnupg \
  lsb-release \
  vim \
  unzip \
  zip \
  htop \
  net-tools \
  ufw \
  fail2ban \
  jq \
  make \
  pkg-config \
  software-properties-common \
  apt-transport-https \
  openssh-server
```

---

## 3. Set `vi` as the system default editor (as root)

```bash
# (as root)
# vim.basic is the update-alternatives entry for vi/vim
update-alternatives --set editor /usr/bin/vim.basic
update-alternatives --set vi     /usr/bin/vim.basic 2>/dev/null || true

# Make EDITOR/VISUAL default to vi for all login shells
cat >/etc/profile.d/editor.sh <<'EOF'
export EDITOR=vi
export VISUAL=vi
EOF
chmod 0644 /etc/profile.d/editor.sh
```

---

## 4. Create the `dev` user (as root)

```bash
# (as root)
adduser --gecos "" --disabled-password dev
# set a password only if you want console/su login (SSH will use keys):
# passwd dev

# Add dev to the sudo group
usermod -aG sudo dev
```

---

## 5. Sudoers — let `dev` run sudo without a password prompt (as root)

> A NOPASSWD dev user is convenient on a controlled dev box. Do **not** copy this
> to production. Always edit sudoers via a `.d` file and validate with `visudo -c`.

```bash
# (as root)
cat >/etc/sudoers.d/90-dev-nopasswd <<'EOF'
dev ALL=(ALL) NOPASSWD:ALL
EOF
chmod 0440 /etc/sudoers.d/90-dev-nopasswd

# Validate — MUST print "parsed OK" before you log out
visudo -c
```

Test (log in as `dev` in a new SSH session):

```bash
# (as dev)
sudo whoami        # should print: root  (no password prompt)
```

---

## 6. SSH hardening + keys for the `dev` user (as root, then dev)

```bash
# (as root) — allow the dev user, harden the daemon
mkdir -p /home/dev/.ssh
chmod 700 /home/dev/.ssh
touch /home/dev/.ssh/authorized_keys
chmod 600 /home/dev/.ssh/authorized_keys
chown -R dev:dev /home/dev/.ssh

# Paste YOUR laptop's public key so you can log in as dev:
vi /home/dev/.ssh/authorized_keys     # add your ~/.ssh/id_ed25519.pub line

# Harden sshd (disable root login + password auth once key login works)
vi /etc/ssh/sshd_config
#   PermitRootLogin no
#   PasswordAuthentication no
#   PubkeyAuthentication yes
systemctl restart ssh
```

---

## 7. Firewall & fail2ban (as root)

```bash
# (as root)
ufw default deny incoming
ufw default allow outgoing
ufw allow OpenSSH
ufw allow 8080/tcp          # ruuma dev API (adjust to your gin port)
ufw --force enable
ufw status verbose

systemctl enable --now fail2ban
```

---

## 8. Install Go (latest) (as root)

```bash
# (as root)
cd /usr/local/src
# Resolve the latest stable Go version automatically:
GO_VERSION="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -1)"
echo "Installing ${GO_VERSION}"
curl -fLO "https://go.dev/dl/${GO_VERSION}.linux-amd64.tar.gz"
rm -rf /usr/local/go
tar -C /usr/local -xzf "/usr/local/src/${GO_VERSION}.linux-amd64.tar.gz"

# System-wide PATH for Go
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
/usr/local/go/bin/go version        # prints the installed Go version
mkdir -p /home/dev/go/bin
```

---

## 9. Go tooling + gin/gorm note (as dev)

`gin` and `gorm` are Go modules — they are added per project with `go get`, not
installed on the OS. Install the developer tooling globally:

```bash
# (as dev)
source /etc/profile.d/go.sh
go install github.com/air-verse/air@latest              # live reload for dev
go install github.com/go-delve/delve/cmd/dlv@latest     # debugger
go install golang.org/x/tools/gopls@latest              # language server
go install honnef.co/go/tools/cmd/staticcheck@latest    # linter
```

Inside the ruuma repo, the frameworks are added like this (reference only):

```bash
# (as dev, inside /home/dev/projects/ruuma once the module exists)
go get github.com/gin-gonic/gin@latest
go get gorm.io/gorm@latest
go get gorm.io/driver/postgres@latest
```

---

## 10. Install PostgreSQL 18 (PGDG repo) (as root)

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
systemctl status postgresql --no-pager
```

Config file locations (full paths):

- Server config: `/etc/postgresql/18/main/postgresql.conf`
- Client auth:   `/etc/postgresql/18/main/pg_hba.conf`
- Data dir:      `/var/lib/postgresql/18/main`
- Logs:          `/var/log/postgresql/postgresql-18-main.log`

### 10.1 Create the ruuma role & database (as root)

```bash
# (as root)
sudo -u postgres psql <<'SQL'
CREATE ROLE ruuma WITH LOGIN PASSWORD '<STRONG_DB_PASSWORD>';
CREATE DATABASE ruuma OWNER ruuma;
GRANT ALL PRIVILEGES ON DATABASE ruuma TO ruuma;
SQL
```

### 10.2 Allow local password auth for the app (as root)

```bash
# (as root)
vi /etc/postgresql/18/main/pg_hba.conf
# Ensure a line exists (local dev, scram):
#   local   ruuma   ruuma                     scram-sha-256
#   host    ruuma   ruuma   127.0.0.1/32      scram-sha-256

vi /etc/postgresql/18/main/postgresql.conf
#   listen_addresses = 'localhost'          # dev: localhost only
#   password_encryption = scram-sha-256

systemctl restart postgresql
```

Test:

```bash
# (as dev)
PGPASSWORD='<STRONG_DB_PASSWORD>' psql -h 127.0.0.1 -U ruuma -d ruuma -c '\conninfo'
```

---

## 11. Git configuration — never re-type your password (as dev)

Two supported methods. **SSH keys are recommended.**

### 11.1 Recommended: SSH key auth to GitHub

```bash
# (as dev)
ssh-keygen -t ed25519 -C "dev@ruuma-dev" -f /home/dev/.ssh/id_ed25519 -N ""

# Start the agent for this session and load the key
eval "$(ssh-agent -s)"
ssh-add /home/dev/.ssh/id_ed25519

# Print the PUBLIC key — add it at https://github.com/settings/keys
cat /home/dev/.ssh/id_ed25519.pub
```

Make the agent auto-start for every login shell:

```bash
# (as dev)
cat >>/home/dev/.bashrc <<'EOF'

# Auto-start ssh-agent and load the GitHub key
if [ -z "$SSH_AUTH_SOCK" ]; then
  eval "$(ssh-agent -s)" >/dev/null
  ssh-add /home/dev/.ssh/id_ed25519 2>/dev/null
fi
EOF

# Test and use SSH remotes (git@github.com:...) so git never asks for a password
ssh -T git@github.com || true
```

### 11.2 Alternative: HTTPS + Personal Access Token cached forever

```bash
# (as dev)
# Store credentials on disk so they are entered exactly once.
git config --global credential.helper 'store'
# First push over HTTPS will prompt once: username = your GitHub user,
# password = a PAT from https://github.com/settings/tokens (repo scope).
# It is then saved to /home/dev/.git-credentials and never asked again.
```

### 11.3 Global git identity & sane defaults (as dev)

```bash
# (as dev)
git config --global user.name  "stevenwilliam"
git config --global user.email "itdept.sfg@gmail.com"
git config --global init.defaultBranch main
git config --global pull.rebase true
git config --global core.editor vi
git config --global core.autocrlf input
git config --global push.autoSetupRemote true
git config --list --show-origin | sed 's/^/  /'
```

---

## 12. Project layout (as dev)

```bash
# (as dev)
mkdir -p /home/dev/projects
cd /home/dev/projects
# SSH clone (method 11.1):
git clone git@github.com:stevenwilliam/ruuma.git
# or HTTPS clone (method 11.2):
# git clone https://github.com/stevenwilliam/ruuma.git
ls -la /home/dev/projects/ruuma
```

Canonical full paths on this server:

- Repo:        `/home/dev/projects/ruuma`
- Go binaries: `/home/dev/go/bin`
- App config:  `/etc/ruuma/ruuma.env`  (created in §14)

---

## 13. CI/CD — self-hosted GitHub Actions runner (as dev, then root)

This puts a runner **on the dev server** so pushes build/test/deploy here.

```bash
# (as dev)
mkdir -p /home/dev/actions-runner
cd /home/dev/actions-runner
# Get the latest runner (check https://github.com/actions/runner/releases for version)
RUNNER_VER="2.320.0"
curl -fLO "https://github.com/actions/runner/releases/download/v${RUNNER_VER}/actions-runner-linux-x64-${RUNNER_VER}.tar.gz"
tar xzf "/home/dev/actions-runner/actions-runner-linux-x64-${RUNNER_VER}.tar.gz"

# Register — get the token from:
# GitHub repo → Settings → Actions → Runners → New self-hosted runner
./config.sh --url https://github.com/stevenwilliam/ruuma --token <RUNNER_TOKEN> --labels ruuma-dev --unattended
```

Install as a systemd service so it survives reboots:

```bash
# (as root)
cd /home/dev/actions-runner
./svc.sh install dev
./svc.sh start
./svc.sh status
```

### 13.1 Example workflow — `/home/dev/projects/ruuma/.github/workflows/ci.yml`

```yaml
name: ruuma-ci
on:
  push:
    branches: [ main ]
jobs:
  build-test-deploy:
    runs-on: [self-hosted, ruuma-dev]
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
        run: /usr/local/bin/ruuma-notify "✅ ruuma CI passed on ${{ github.sha }}"
      - name: WhatsApp notify (failure)
        if: failure()
        run: /usr/local/bin/ruuma-notify "❌ ruuma CI FAILED on ${{ github.sha }}"
```

---

## 14. WhatsApp notify & interaction (as root, then dev)

Goal: the dev server (and CI/CD) can **send** WhatsApp messages, and optionally
**receive/interact** for ChatOps. This uses the official Meta WhatsApp Cloud API.
(Alternatives: Twilio WhatsApp, or a self-hosted `whatsapp-web.js` bridge.)

### 14.1 Store secrets (as root)

```bash
# (as root)
mkdir -p /etc/ruuma
cat >/etc/ruuma/whatsapp.env <<'EOF'
# Meta WhatsApp Cloud API — from https://developers.facebook.com/ (WhatsApp product)
WHATSAPP_TOKEN=<META_PERMANENT_TOKEN>
WHATSAPP_PHONE_ID=<WHATSAPP_BUSINESS_PHONE_NUMBER_ID>
WHATSAPP_TO=<YOUR_DESTINATION_NUMBER_E164>   # e.g. 6281234567890
WHATSAPP_VERIFY_TOKEN=<PICK_A_RANDOM_STRING>  # for inbound webhook verification
EOF
chmod 0640 /etc/ruuma/whatsapp.env
chown root:dev /etc/ruuma/whatsapp.env
```

### 14.2 Send helper — `/usr/local/bin/ruuma-notify` (as root)

```bash
# (as root)
cat >/usr/local/bin/ruuma-notify <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
set -a; source /etc/ruuma/whatsapp.env; set +a
MSG="${*:-ruuma notification}"
curl -fsS -X POST \
  "https://graph.facebook.com/v20.0/${WHATSAPP_PHONE_ID}/messages" \
  -H "Authorization: Bearer ${WHATSAPP_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "$(jq -n --arg to "$WHATSAPP_TO" --arg body "$MSG" \
        '{messaging_product:"whatsapp", to:$to, type:"text", text:{body:$body}}')"
EOF
chmod 0755 /usr/local/bin/ruuma-notify
```

Test:

```bash
# (as dev)
/usr/local/bin/ruuma-notify "ruuma-dev server is up 🚀"
```

### 14.3 Interaction / ChatOps (inbound)

For two-way interaction, the ruuma service exposes a webhook that Meta calls on
inbound messages. Configure the webhook in the Meta dashboard to point at:

```
https://<YOUR_DEV_HOSTNAME>/api/v1/webhooks/whatsapp
```

- Verification uses `WHATSAPP_VERIFY_TOKEN` (GET challenge).
- Inbound messages POST here; the handler routes commands (e.g. "deploy",
  "status") — implemented in `internal/adapter/notify` once the app exists.
- For local dev without a public IP, tunnel with `cloudflared` or `ngrok`.

> Full implementation of the inbound handler is a code task tracked in the
> roadmap; this section prepares the server and secrets only.

---

## 15. Verification checklist

```bash
# (as dev)
/usr/local/go/bin/go version                                   # Go installed
psql -h 127.0.0.1 -U ruuma -d ruuma -c '\conninfo'             # DB reachable
sudo whoami                                                    # NOPASSWD sudo works
ssh -T git@github.com || true                                  # git SSH auth
git -C /home/dev/projects/ruuma remote -v                      # repo cloned
sudo systemctl status postgresql --no-pager                    # PG running
sudo /home/dev/actions-runner/svc.sh status                    # CI runner up
/usr/local/bin/ruuma-notify "checklist complete ✅"            # WhatsApp send
echo "EDITOR=$EDITOR"                                          # should be vi
```

Server is ready for `go run ./cmd/api serve` once the ruuma code lands (see
`/home/dev/projects/ruuma/docs/RUN-WHEN-BACK.md`).
