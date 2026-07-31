# Run when you're back — interactive verification

Steps that need an interactive terminal (Docker, a live server, approval
prompts). Run them top to bottom in a shell from the repo root. Use `vi` for any
edits.

> Nothing to run yet — the service isn't scaffolded. This file fills in as M1
> lands. The template below is the shape it will take.

## 1. Prerequisites (one-time)

```bash
cp .env.example .env
```

## 2. Start the local stack

```bash
docker compose up -d
docker compose ps            # all healthy
```

## 3. Migrate + seed

```bash
go run ./cmd/api migrate
```

## 4. Run + smoke test

```bash
go run ./cmd/api serve
# in another shell:
curl -s localhost:8080/health
```
