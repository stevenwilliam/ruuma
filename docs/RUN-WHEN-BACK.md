# Run when you're back — interactive verification

Steps that need an interactive terminal, a browser, or credentials that do not
exist yet. Everything else has already been run; what is here is what could not
be, and why. Use `vi` for any edits.

_Updated: 2026-08-12._

## Already done — no action needed

These ran during the build and are reported in `docs/12-security.md` §6:

- `go test ./...`, integration, security and e2e suites — all green
- `go vet`, `staticcheck`, `gosec`, `govulncheck` — clean
- Migrations up → down → up on an empty database
- `cmd/api seed` and `cmd/api create-owner` against a live PostgreSQL 18
- The API served and smoke-tested over HTTP
- Frontend typecheck, lint, unit tests and production build

## 1. See the whole thing running locally

```bash
cd /home/dev/projects/ruuma
docker compose -f /home/dev/projects/ruuma/docker-compose.yml up -d   # MinIO + mailpit
make migrate
make seed
make run          # API on 127.0.0.1:8080
```

In a second terminal:

```bash
cd /home/dev/projects/ruuma
make web-dev      # 127.0.0.1:5173
```

`claudedev` is headless, so `127.0.0.1:5173` is only reachable from the box
itself. Tunnel it from the machine that has the browser:

```bash
ssh -L 5173:127.0.0.1:5173 dev@192.168.88.101   # then browse http://localhost:5173
```

`make web-dev-lan` binds `0.0.0.0` instead, but then ufw has to allow the port
from the laptop's *NAT source address* — which is `172.16.0.1`, not the server's
own `192.168.88.0/24` subnet. `docs/11-local-dev-setup.md` §6 has the detail;
the tunnel is the less fragile option.

The seed prints a generated staff password — sign in to the admin at
`/admin` with `owner@ruuma.id` and that password. It is shown once and not
stored, so if it has scrolled away, re-run `make seed` with `SEED_PASSWORD` set
in the environment, or mint a fresh account with `cmd/api create-owner`.

## 2. Exercise the object-storage path by hand

The automated suites use an in-memory stand-in, so the real MinIO path is the
one gap worth walking (`docs/12` §7):

1. Place an order in the customer UI.
2. Upload a payment proof — try a `.png` that is really an HTML file, and a file
   over 5 MB; both must be refused.
3. In the admin finance queue, open the proof: the link must be presigned and
   expire.

## 3. WhatsApp OTP — the session needs a QR scan (BLOCKING)

**This is the one thing stopping WhatsApp OTP from working.** Everything else in
the chain is fixed and proven: `notify.provider` is `waha`, `WAHA_API_KEY` is
populated from `/etc/claudedev/whatsapp.env`, and the worker drains the queue
and reaches WAHA. The queue now fails with WAHA's own message:

```
waha: status 422: {"error":"Session status is not as expected. Try again later
or restart the session"}
```

The stored WhatsApp Web credentials for session `default` are dead — the phone
unlinked it or it expired. A restart does not recover it; it goes
`STARTING` → `FAILED`. Re-pairing needs the QR scanned from the handset that
owns **+62 817-6315-568**, which cannot be automated.

```bash
# 1. Confirm the session is still failed
KEY=$(sudo grep -oP '(?<=^WAHA_API_KEY=).*' /etc/claudedev/whatsapp.env)
curl -s -H "X-Api-Key: $KEY" http://127.0.0.1:3000/api/sessions/default | python3 -m json.tool

# 2. Restart it, then immediately fetch the pairing QR
curl -s -X POST -H "X-Api-Key: $KEY" http://127.0.0.1:3000/api/sessions/default/restart
curl -s -H "X-Api-Key: $KEY" http://127.0.0.1:3000/api/default/auth/qr?format=image \
  -o /tmp/waha-qr.png

# 3. Open /tmp/waha-qr.png over an SSH tunnel or scp it, then on the phone:
#    WhatsApp → Settings → Linked devices → Link a device → scan.

# 4. It should report WORKING
curl -s -H "X-Api-Key: $KEY" http://127.0.0.1:3000/api/sessions/default | python3 -m json.tool
```

Then re-drive an OTP and watch it leave:

```bash
curl -s -X POST http://127.0.0.1/api/v1/otp/request \
  -H 'Content-Type: application/json' \
  -d '{"phone":"628176315568","purpose":"login"}'

sudo -u postgres psql -d ruuma -c \
  "SELECT channel, status, attempts, left(last_error,80) FROM notifications ORDER BY created_at DESC LIMIT 3;"
```

`status = sent` is the finish line. Any queued notification that failed earlier
stays `failed` — retry or delete those rows, they will not be picked up again
once attempts are exhausted.

## 3a. The worker has to be running, and it was not

Notifications sit in `queued` forever unless `cmd/api worker` is running — it is
what drains the queue on a 15-second tick. It was not running, which is why
nothing was even attempted.

Both the API and the worker are currently started by hand
(`nohup setsid go run ./cmd/api …`) and **will not survive a reboot**. They have
been restarted by hand several times during this build, and twice a stale
process kept the port and silently served old configuration. They want systemd
units:

```bash
sudo vi /etc/systemd/system/ruuma-api.service
sudo vi /etc/systemd/system/ruuma-worker.service
sudo systemctl daemon-reload
sudo systemctl enable --now ruuma-api ruuma-worker
```

Until then, after changing `.env`, confirm the *serving* process actually picked
it up rather than assuming the restart worked:

```bash
PID=$(sudo ss -ltnp | grep 8080 | grep -oP 'pid=\K[0-9]+' | head -1)
sudo tr '\0' '\n' < /proc/$PID/environ | grep APP_BASE_URL
```

## 4. Google and Instagram sign-in

Blocked on credentials (`docs/00` Q8). Both flows are built and refuse cleanly
while disabled. When the OAuth apps exist:

1. Put the client id/secret in `/home/dev/projects/ruuma/.env`.
2. Admin → Parameters → `auth.provider_google_enabled` = `true`.
3. Note that Instagram returns no email or phone, so those customers must add
   and verify a phone before their first order.

## 5. Production deployment

`docs/14-production-deployment-handbook.md` is written for an empty machine and
has not been executed — there is no production server yet. Work through it top
to bottom, then the go-live checklist in §13.

## 6. Load and latency

The p95 targets in `docs/05-architecture-and-nfr.md` §4 have not been measured.
Worth doing before the first busy Friday: `k6` or `hey` against
`/api/v1/menu` and `/api/v1/availability/slots` at 100 concurrent.
