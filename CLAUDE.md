# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Volt pulls daily electricity consumption from the EVNHCMC customer portal, upserts it into PostgreSQL, computes tiered billing cost (with VAT), and reports it through a Telegram bot and a small HTTP API. Module path: `github.com/gamee1910/volt`.

## Commands

```bash
docker compose up -d db            # local Postgres 16 (admin/password, db "volt", port 5432)
make migrate-up                    # apply migrations (golang-migrate CLI; DB_URL set in Makefile)
make migrate-create name=foo       # new sequential migration in ./migrations
make migrate-down-one | migrate-version | migrate-force version=N
make gen-docs                      # swag docs from cmd/api/main.go into ./docs

air                                # run API with live reload (builds ./cmd/api to ./bin/main)
go run ./cmd/api                   # API server + Telegram bot + in-process scheduler
go run ./cmd/cli/sync              # one-shot sync: first of month -> today
go run ./cmd/cli/report            # one-shot: send yesterday's report to the bot's first chat

go build ./...
go vet ./...
go test ./...                      # unit tests live in internal/application
go test ./internal/application -run TestYesterday   # single test
```

Config comes only from environment variables (`config/config.go`), normally loaded via direnv from `.envrc` (template: `.envrc.example`). Note the customer code variable is `EVN_CUSTOMER` (the README's `EVN_CUSTOMER_CODE` is outdated). The GitHub workflows in `.github/workflows/` run the CLI commands on a self-hosted runner and are `workflow_dispatch` only (their cron triggers are commented out).

## Architecture

Layered layout (domain → application → infrastructure / interfaces), no CQRS:

- `internal/domain` — entities (`entity.ElectricityConsumption`) and the `repository.ElectricityRepository` interface. No outward dependencies.
- `internal/application` — one `ElectricityService` with plain methods: `Sync(from, to)`, `SyncCurrentMonth`, `CurrentMonthRange`, `Yesterday`, `All`. Add new use cases as methods here. `types.go` holds the DTOs it returns (no JSON tags, so field names are the API JSON keys). `Sync` logs start/finish with duration. Tests use in-memory fakes for the repository and EVN client, and override `now` for date logic.
- `internal/common` — `utils` (Vietnam timezone, EVN date format/parse, `CalculateElectricityBill` tier pricing, VND formatting).
- `internal/infrastructure` — `persistences/postgres` implements the repository with `database/sql` + `lib/pq`. `di/container.go` is the single composition root: it builds the EVN and Telegram clients, repository, `ElectricityService`, HTTP handler, bot command handler, scheduler, and attaches the bot router as the Telegram message handler.
- `internal/interfaces` — the three entry adapters, all of which call into `ElectricityService` only:
  - `api` — gin router (`/health`, `/api/v1/{sync,"",yesterday}`) and handlers. `gin.ReleaseMode` is set in `cmd/api` when `APP_ENV=production`.
  - `bot` — Telegram command router (`/yesterday`, `/sync [from] [to]`, `/get`, `/help`); messages are in Vietnamese.
  - `scheduler` — `robfig/cron` job (`0 */12 * * *`, Asia/Ho_Chi_Minh) that runs `SyncCurrentMonth`, plus once immediately on `Start()`. After each cron run (not the startup run) it reports to `TELEGRAM_CHAT_ID` via `ElectricityCommand.NotifySyncResult`; unset/0 disables this.
- `pkg/` — reusable clients with no dependency on `internal`: `evnhcmc` (portal login + daily usage API), `telegram` (wrapper around `go-telegram/bot`), `logger` (zap).
- `cmd/` — `api` (long-running server) and `cli/{sync,report}` (one-shot jobs). Each entrypoint builds config → logger → DB → `di.NewContainer`, so all three share the same wiring.

Domain details worth knowing:
- All date logic uses the Vietnam timezone via `utils.LoadVietnamTimezone()` (tzdata is embedded via `time/tzdata`, so it works on the alpine runtime image); sync ranges are always "first day of month → now" unless given explicitly.
- Sync upserts one row per `measurement_date` (unique constraint). If EVN reports `TotalOutput` of 0, `Sync` falls back to `TotalIndex`.
- Monthly cost = `CalculateElectricityBill(total kWh from first of month to yesterday)`.
- `docs/mock_evnhcmc.json` is a sample EVN API response.

## Docs in `.claude/`

- `.claude/project-summary.md` — project overview and known gaps (Vietnamese).
- `.claude/deploy-checklist.md` — step-by-step Botkeep deploy checklist (Vietnamese).
