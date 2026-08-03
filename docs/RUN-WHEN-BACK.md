# Run when you're back — interactive verification

Steps that need an interactive terminal, a browser, or credentials that do not
exist yet. Everything else has already been run; what is here is what could not
be, and why. Use `vi` for any edits.

_Updated: 2026-08-02._

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

## 3. Send a real WhatsApp message

Notifications default to the `log` provider so a local run never touches the
shared WAHA session. To test a real send, to your own number only:

```bash
# Admin → Parameters → notify.provider = waha
make worker
```

Then place an order and watch `journalctl`/stdout for the dispatch.

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
