# Tickordo

[![Backend CI](https://github.com/pitercoding/tickordo/actions/workflows/backend-ci.yml/badge.svg)](https://github.com/pitercoding/tickordo/actions/workflows/backend-ci.yml)
[![Frontend CI](https://github.com/pitercoding/tickordo/actions/workflows/frontend-ci.yml/badge.svg)](https://github.com/pitercoding/tickordo/actions/workflows/frontend-ci.yml)
![Go](https://img.shields.io/badge/Go-1.26.2-00ADD8?logo=go&logoColor=white)
![Next.js](https://img.shields.io/badge/Next.js-16.4.0-000000?logo=nextdotjs&logoColor=white)
![React](https://img.shields.io/badge/React-19.3.0-61DAFB?logo=react&logoColor=black)
![TypeScript](https://img.shields.io/badge/TypeScript-5-3178C6?logo=typescript&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-18-4169E1?logo=postgresql&logoColor=white)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Tickordo** is an AI-powered support ticket management and triage application built with **Go**, **Next.js**, and **PostgreSQL**.

Support teams spend a lot of time reading incoming tickets before they can decide what each one is about, how urgent it is, and who should handle it. Tickordo lets you create and track support tickets and, on request, asks an OpenAI model to suggest a category, priority, sentiment, responsible team, summary, and next action for a ticket. Every analysis is stored, so a ticket keeps a history of its AI triage results. The goal is to assist support agents, not to replace them.

> **This is a portfolio project.** It demonstrates REST API design in Go, PostgreSQL persistence, a Next.js frontend integrated with the API, automated testing and CI, cloud deployment, and a cost-bounded integration with an external AI API. The public demo contains **fictional tickets only**, has **no authentication**, and is not a production-ready support platform.

| | URL |
| --- | --- |
| **Live demo (frontend)** | [tickordo.vercel.app](https://tickordo.vercel.app/) |
| **Backend API** | [tickordo-backend.onrender.com](https://tickordo-backend.onrender.com/) |
| **Health check** | [tickordo-backend.onrender.com/health](https://tickordo-backend.onrender.com/health) |
| **Repository** | [github.com/pitercoding/tickordo](https://github.com/pitercoding/tickordo) |

The frontend is hosted on Vercel and the Go API on Render's free tier. A free Render service can spin down when idle, so the first request after a quiet period may be noticeably slow while the API starts up again.

![Tickordo dashboard showing open, high-priority and AI-analyzed ticket counters and a "Needs attention" list of unresolved critical and high priority tickets](docs/screenshots/dashboard.png)

## Table of contents

- [Features](#features)
- [Screenshots](#screenshots)
- [Tech stack](#tech-stack)
- [Architecture](#architecture)
- [Project structure](#project-structure)
- [Getting started](#getting-started)
- [Environment variables](#environment-variables)
- [Database and migrations](#database-and-migrations)
- [AI integration](#ai-integration)
- [Testing](#testing)
- [Continuous integration](#continuous-integration)
- [Production deployment](#production-deployment)
- [Security and operational safeguards](#security-and-operational-safeguards)
- [API](#api)
- [Technical decisions](#technical-decisions)
- [Known limitations](#known-limitations)
- [Project status](#project-status)
- [License](#license)

## Features

### Tickets

- **Create tickets** with a title (up to 255 characters) and description (up to 5,000 characters). Limits are enforced by the API and mirrored in the form.
- **Ticket list** with search by title, filters by status, AI priority, and AI category, sorting (newest, oldest, title A–Z / Z–A), and pagination (10 tickets per page). Filtering, sorting, and pagination happen in the browser over the full list returned by the API.
- **Ticket details** page with description, status, and creation/update timestamps formatted in the browser's timezone.
- **Status management**: `open`, `in_progress`, or `resolved`, validated by the API and by a database `CHECK` constraint.

### AI triage

- **Analyze with AI** from the ticket details page. The result shows category, priority (`low`, `medium`, `high`, `critical`), sentiment (`positive`, `neutral`, `negative`, `frustrated`), suggested team, summary, suggested action, confidence (0–1, shown as a percentage), the model used, and the analysis time.
- **Re-analysis and history**: a ticket can be analyzed again; the latest result is shown first and earlier results are listed under "Previous analyses".
- The ticket list and dashboard use each ticket's **latest** triage for its category and priority.

### Dashboard

- Counters for **open tickets**, **high priority** (unresolved tickets whose latest triage is `high` or `critical`), and **AI analyzed** (tickets with at least one triage).
- A **Needs attention** section listing up to five unresolved critical/high tickets, most urgent first.
- A time-of-day greeting based on the browser's local clock.

### User experience

- Loading skeletons while data is fetched, inline error alerts per dashboard section, a not-found page for unknown ticket IDs, and a page-level error boundary with a retry button.
- API validation messages (4xx) are shown to the user; server errors fall back to generic messages.
- Requests time out on the client (15 seconds by default, 75 seconds for AI triage).

## Screenshots

| Ticket list | Ticket details |
| --- | --- |
| ![Ticket list with search, status, priority and category filters, sort order, and a table showing each ticket's status, AI category and AI priority](docs/screenshots/ticket-list.png) | ![Ticket details page with the status selector, full description, and created and last updated timestamps](docs/screenshots/ticket-details.png) |

| AI triage result | Triage history |
| --- | --- |
| ![AI triage panel showing category, priority, sentiment, suggested team, summary, suggested action, model, analysis time and confidence](docs/screenshots/ai-triage-result.png) | ![AI triage panel with the latest analysis and a collapsed entry under "Previous analyses"](docs/screenshots/triage-history.png) |

## Tech stack

Versions are taken from `backend/go.mod`, `frontend/package.json`, `frontend/package-lock.json`, the Docker files, and the CI workflows.

| Area | Technology |
| --- | --- |
| **Frontend** | Next.js 16.4.0 (App Router, React Server Components), React 19.3.0, TypeScript 5, Tailwind CSS 4 |
| **Backend** | Go 1.26.2, standard library `net/http` (`http.ServeMux` with method and path patterns) |
| **Database** | PostgreSQL 18, [pgx](https://github.com/jackc/pgx) v5.11.0 (`pgxpool`) |
| **AI integration** | [openai-go](https://github.com/openai/openai-go) v3.71.2, OpenAI Responses API with structured output |
| **Other Go libraries** | `google/uuid` v1.6.0, `joho/godotenv` v1.5.1 |
| **Testing** | Go `testing` package, Vitest 4 |
| **Linting** | ESLint 9 with `eslint-config-next`, `go vet`, `tsc --noEmit` |
| **Migrations** | SQL migration files, [golang-migrate](https://github.com/golang-migrate/migrate) v4.20.1 via a Docker image |
| **Infrastructure** | Docker Compose (local PostgreSQL), GitHub Actions, Vercel (frontend), Render (API) |

## Architecture

```text
                         ┌──────────────────────────────┐
  Browser ──────────────▶│  Next.js frontend (Vercel)   │
     │                   │  Server Components fetch     │
     │                   │  dashboard, list and details │
     │                   └──────────────┬───────────────┘
     │  client-side calls               │ server-side GET requests
     │  (create, status, triage)        ▼
     │                   ┌──────────────────────────────┐        ┌──────────────┐
     └──────────────────▶│  Go REST API (Render)        │───────▶│  OpenAI API  │
                         │  CORS → rate limit → handler │        │  (Responses) │
                         │  → service → repository      │        └──────────────┘
                         └──────────────┬───────────────┘
                                        ▼
                         ┌──────────────────────────────┐
                         │  PostgreSQL                  │
                         └──────────────────────────────┘
```

- **Reads** for the dashboard, ticket list, and ticket details are performed by Next.js Server Components, which call the API from the server.
- **Interactive actions** (creating a ticket, changing status, loading triage history, and requesting a triage) are performed by client components that call the API directly from the browser, which is why the API sends CORS headers for a single configured origin.
- Only the Go API talks to PostgreSQL and OpenAI. The browser never receives the OpenAI key or database credentials.

The backend is organized in layers:

| Layer | Package | Responsibility |
| --- | --- | --- |
| Routing | `internal/routes` | Registers routes on `http.ServeMux` and attaches per-route rate limiting |
| Middleware | `internal/middleware` | CORS, rate limiting, client IP resolution behind trusted proxies |
| Handlers | `internal/handlers` | HTTP concerns: decoding and validating input, mapping errors to status codes, JSON responses |
| Services | `internal/services` | Use cases: ticket operations, triage orchestration, dashboard stats, AI concurrency and quota limits |
| AI | `internal/ai` | OpenAI client configuration, prompt, JSON schema, and response validation |
| Repositories | `internal/repositories` | SQL queries with pgx |
| Rate limiting | `internal/ratelimit` | In-memory fixed-window limiters (per key and global) |
| Config | `internal/config` | Environment variable parsing and validation at startup |

Services depend on small interfaces rather than concrete repositories, which lets most handler and service tests run with fakes instead of a database or OpenAI.

## Project structure

```text
tickordo/
├── .github/workflows/
│   ├── backend-ci.yml          # Go tests (with PostgreSQL), vet, build
│   └── frontend-ci.yml         # Vitest, ESLint, type check, build
├── backend/
│   ├── cmd/api/main.go         # Entry point: config, wiring, HTTP server, graceful shutdown
│   ├── internal/
│   │   ├── ai/                 # OpenAI analyzer
│   │   ├── config/             # Environment configuration
│   │   ├── database/           # pgx connection pool
│   │   ├── handlers/           # HTTP handlers
│   │   ├── middleware/         # CORS, rate limit, client IP
│   │   ├── models/             # Ticket, TicketTriage, DashboardStats
│   │   ├── ratelimit/          # In-memory limiters
│   │   ├── repositories/       # PostgreSQL queries + integration tests
│   │   ├── routes/             # Route registration
│   │   └── services/           # Business logic
│   └── go.mod
├── frontend/
│   ├── app/                    # Dashboard, /tickets/new, /tickets/[id]
│   ├── components/             # Dashboard, ticket and UI components
│   ├── lib/api/                # API client and typed endpoint functions
│   └── package.json
├── migrations/                 # Versioned SQL migrations (up/down)
├── docker/migrate.Dockerfile   # Manual migration runner (golang-migrate)
├── docker-compose.yml          # Local PostgreSQL
├── docs/screenshots/
├── .env.example                # Backend environment template
└── LICENSE
```

## Getting started

### Prerequisites

- Go 1.26.2 or a compatible toolchain (the version declared in `backend/go.mod`)
- Node.js and npm (CI uses Node.js 22)
- Docker with Docker Compose

### 1. Clone the repository

```bash
git clone https://github.com/pitercoding/tickordo.git
cd tickordo
```

### 2. Configure the backend environment

```bash
cp .env.example .env
```

The API loads `../.env` relative to its working directory, so the root `.env` is picked up when the API is run from `backend/`. The template points `DATABASE_URL` at the Docker Compose database. `OPENAI_API_KEY` must be non-empty for the API to start; AI triage only works with a valid key, and every triage is a billed OpenAI request. All other features work without one.

`.env` files are git-ignored. Never commit real keys or credentials.

### 3. Start PostgreSQL

```bash
docker compose up -d
```

This starts PostgreSQL 18 (`tickordo-postgres`) on **host port 5433** with database, user, and password `tickordo` — local development values only.

### 4. Apply the migrations

Build the migration image from the repository root and run it against the local database:

```bash
docker build -f docker/migrate.Dockerfile -t tickordo-migrate .

DATABASE_URL='postgres://tickordo:tickordo@host.docker.internal:5433/tickordo?sslmode=disable' \
  docker run --rm -e DATABASE_URL tickordo-migrate up
```

The container reaches the host database through `host.docker.internal`, which Docker Desktop provides. On Linux, add `--add-host=host.docker.internal:host-gateway` to `docker run`.

### 5. Run the API

```bash
cd backend
go run ./cmd/api
```

The API listens on `http://localhost:8080` by default. Verify it with:

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

### 6. Run the frontend

In another terminal:

```bash
cd frontend
npm install
npm run dev
```

Open `http://localhost:3000`. In development the frontend defaults to `http://localhost:8080` for the API; to change it, copy `frontend/.env.example` to `frontend/.env.local` and edit `NEXT_PUBLIC_API_URL`.

## Environment variables

### Backend (`.env` at the repository root, or the host environment)

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `DATABASE_URL` | Yes | — | PostgreSQL connection string |
| `OPENAI_API_KEY` | Yes | — | OpenAI API key used for triage |
| `PORT` | No | `8080` | HTTP port |
| `CORS_ALLOWED_ORIGIN` | No | `http://localhost:3000` | The single origin allowed to call the API from a browser |
| `OPENAI_MODEL` | No | `gpt-5.6` | Model sent to the Responses API and stored with each triage |
| `OPENAI_REASONING_EFFORT` | No | unset (model default) | One of `none`, `minimal`, `low`, `medium`, `high`, `xhigh`, `max` |
| `RATE_LIMIT_TICKETS_REQUESTS` | No | `20` | Ticket creations allowed per client IP per window |
| `RATE_LIMIT_TICKETS_WINDOW` | No | `10m` | Window for ticket creation (Go duration) |
| `RATE_LIMIT_TRIAGE_REQUESTS` | No | `5` | Triage requests allowed per client IP per window |
| `RATE_LIMIT_TRIAGE_WINDOW` | No | `10m` | Window for triage requests |
| `TRIAGE_GLOBAL_LIMIT` | No | `100` | Triages allowed across all clients per global window |
| `TRIAGE_GLOBAL_WINDOW` | No | `24h` | Window for the global triage quota |
| `TRIAGE_MAX_CONCURRENT` | No | `3` | Maximum triages running at the same time |
| `TRUSTED_PROXY_CIDRS` | No | empty | Comma-separated CIDRs whose `X-Forwarded-For` entries are trusted; empty means only the TCP peer address is used |

Configuration is validated at startup: missing required variables, non-positive limits, invalid durations, unknown reasoning efforts, and overly broad proxy ranges (broader than `/8` for IPv4 or `/32` for IPv6) stop the API with an error.

### Frontend (`frontend/.env.local`, or the Vercel project settings)

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `NEXT_PUBLIC_API_URL` | In production builds | `http://localhost:8080` in development | Base URL of the Go API. Inlined at build time; a production build fails if it is missing |

### Tests

| Variable | Purpose |
| --- | --- |
| `TEST_DATABASE_URL` | PostgreSQL URL for the repository integration tests. Must point to a database named `tickordo_test` |

## Database and migrations

The schema is defined by the SQL files in [`migrations/`](migrations):

| Migration | Change |
| --- | --- |
| `001_create_tickets` | `tickets` table: UUID primary key, `title VARCHAR(255)`, `description TEXT`, `status` (default `open`), `created_at`, `updated_at` |
| `002_create_ticket_triages` | `ticket_triages` table: UUID primary key, `ticket_id` foreign key to `tickets`, category, priority, sentiment, suggested team, summary, suggested action, confidence, model, `created_at`; index on `ticket_id` |
| `003_add_ticket_status_check` | `CHECK` constraint limiting `tickets.status` to `open`, `in_progress`, `resolved` |

Each migration has an `up` and a `down` file. Triage results are append-only: a new analysis inserts a row, and the latest row per ticket (by `created_at`, then `id`) is treated as the current triage.

### Migration runner

Migrations are applied **manually** with the image built from [`docker/migrate.Dockerfile`](docker/migrate.Dockerfile), which wraps golang-migrate v4.20.1:

- Only `migrations/*.sql` is copied into the image; the build context allowlist keeps `.env` and other files out.
- No credentials are stored in the image. `DATABASE_URL` is read from the environment at run time, and the container refuses to run a command without it.
- Nothing runs at build time or without an explicit command (`up`, `down`, `version`, ...).
- URLs are redacted from the error output, and the exit code of `migrate` is preserved.

```bash
docker build -f docker/migrate.Dockerfile -t tickordo-migrate .
docker run --rm -e DATABASE_URL tickordo-migrate version
```

The API does not run migrations on startup. Applying migrations to the production database is a deliberate operator action; it is not part of CI or deployment, and the production database should never be used for local development or tests.

## AI integration

When a triage is requested (`POST /tickets/{id}/triage`), the API:

1. Loads the ticket from PostgreSQL (unknown tickets return `404` without calling OpenAI).
2. Takes a concurrency slot and a unit of the global triage quota (see [safeguards](#security-and-operational-safeguards)).
3. Sends the ticket title and description to the **OpenAI Responses API** with:
   - a **strict JSON schema** (`ticket_triage`) defining every field, with enums for priority and sentiment and no additional properties;
   - the model from `OPENAI_MODEL` and, if set, `OPENAI_REASONING_EFFORT`;
   - `max_output_tokens` set to 4,096 to bound the cost of each response;
   - `store: false`, so OpenAI is asked not to store the responses;
   - a 60-second timeout, and at most one SDK retry with a retry delay capped at 5 seconds.
4. Rejects incomplete responses, refusals (no output text), and invalid values (unknown enums, empty fields, confidence outside 0–1). Category and suggested team are truncated to fit their columns.
5. Stores the result with the model name and returns it with `201 Created`.

Token usage (input, cached, output, reasoning, and total tokens) is logged for every response, using only identifiers and counts, never ticket or response content.

AI output is a suggestion: categories, priorities, and confidence values can be wrong or inconsistent between runs, and the confidence score is reported by the model itself rather than calibrated. Each triage is a paid OpenAI request, so avoid triggering triages just to validate a local setup.

## Testing

### Backend

```bash
cd backend
go test ./...
go vet ./...
go build ./...
```

Unit tests cover configuration parsing, handlers, request decoding, CORS, client IP resolution, rate limiting, routes, services, and the OpenAI analyzer. External dependencies are replaced with fakes; the analyzer tests do not call the real OpenAI API.

Repository integration tests run against a real PostgreSQL database:

- If `TEST_DATABASE_URL` is not set, they are **skipped** locally. When the `CI` environment variable is set, a missing `TEST_DATABASE_URL` **fails** the run instead of skipping silently.
- The tests refuse to run unless the URL **and** the connected server-side database are both named `tickordo_test`, which protects the development and production databases.
- Before running, the test setup **drops and recreates the schema** of that database by applying every `down` migration and then every `up` migration.

To run them locally with the Docker Compose instance:

```bash
docker exec tickordo-postgres createdb -U tickordo tickordo_test

cd backend
TEST_DATABASE_URL='postgres://tickordo:tickordo@localhost:5433/tickordo_test?sslmode=disable' \
  go test -count=1 ./...
```

### Frontend

```bash
cd frontend
npm test            # Vitest
npm run lint        # ESLint
npx tsc --noEmit    # Type check
npm run build       # Production build (requires NEXT_PUBLIC_API_URL)
```

Vitest covers the API client (URL resolution, error handling, timeouts), the greeting logic, and local date/time formatting.

## Continuous integration

Two GitHub Actions workflows run on pushes and pull requests to `main`, each filtered by path:

| Workflow | Triggered by changes to | Steps |
| --- | --- | --- |
| [Backend CI](.github/workflows/backend-ci.yml) | `backend/**`, `migrations/**`, the workflow file | Go from `backend/go.mod`; `go test -count=1 ./...` with a disposable PostgreSQL 18 service (`tickordo_test`) for integration tests; `go vet ./...`; `go build ./...` |
| [Frontend CI](.github/workflows/frontend-ci.yml) | `frontend/**`, the workflow file | Node.js 22; `npm ci`; `npm test`; `npm run lint`; `npx tsc --noEmit`; `npm run build` with a placeholder `NEXT_PUBLIC_API_URL` that is never called |

Both workflows use read-only repository permissions. CI does not deploy the application or run migrations against any hosted database.

## Production deployment

| Component | Platform | Configuration |
| --- | --- | --- |
| Frontend | Vercel | `NEXT_PUBLIC_API_URL` set to the Render API URL. Because the variable is inlined at build time, changing it requires a new build |
| API | Render (free tier) | `DATABASE_URL`, `OPENAI_API_KEY`, and `CORS_ALLOWED_ORIGIN` set to the Vercel origin; optional model, rate limit, and proxy settings as listed in [Environment variables](#environment-variables) |
| Database | Hosted PostgreSQL | Schema applied manually with the migration runner |

Deployment settings live in the Vercel and Render dashboards; the repository does not contain platform-specific deployment files. Secrets are configured as environment variables in each platform and are never committed.

Production migrations are run manually by the operator with the migration image, passing the production `DATABASE_URL` from the local shell environment (for example, check the current version first with `version` before running `up`). They are never applied automatically.

The API shuts down gracefully on `SIGINT`/`SIGTERM`, allowing up to 70 seconds for in-flight requests (including running triages) to finish.

## Security and operational safeguards

> The public demo has **no authentication**: anyone can read, create, and update tickets and request triages. It must contain **fictional data only**. Tickordo is not suitable for real customer tickets or personal data.

Implemented safeguards:

- **Input validation**: JSON bodies are limited to 1 MB (`413` beyond that), unknown fields and trailing data are rejected, title/description are trimmed and length-checked, NUL characters are rejected, and IDs must be valid UUIDs. The triage endpoint rejects any request body.
- **Database integrity**: parameterized queries, `NOT NULL` columns, a foreign key from triages to tickets, and a `CHECK` constraint on ticket status.
- **Error handling**: errors are returned as `{"error": "..."}` with generic messages for server errors; details are only logged.
- **Rate limiting**: per-client-IP fixed-window limits on ticket creation and triage requests, returning `429` with a `Retry-After` header. Read endpoints and status updates are not rate limited.
- **Trusted proxies**: `X-Forwarded-For` is only read when the TCP peer is inside `TRUSTED_PROXY_CIDRS`, walked from right to left, and overly broad ranges are rejected at startup.
- **AI cost controls**: a global triage quota (`429` with `Retry-After` when exhausted), a concurrency cap (`503` with `Retry-After` when all slots are busy), a per-response output token cap, a request timeout, and a bounded retry policy.
- **OpenAI data handling**: `store: false` on every request, and usage logs contain only IDs and token counts.
- **CORS**: a single allowed origin, `GET`/`POST`/`PATCH`/`OPTIONS` methods, and only the `Content-Type` header.
- **HTTP server timeouts**: read-header, read, write, and idle timeouts on the Go server, plus bounded startup and shutdown.
- **Secrets**: `.env` files are git-ignored, the migration image contains no credentials, and the production frontend build fails rather than defaulting to `localhost`.
- **Health check**: `GET /health` for liveness monitoring.

Limitations of these controls: rate limits, the global quota, and concurrency slots live **in memory**, so they reset when the process restarts and are counted **per process** (multiple instances would each have their own limits). The health check does not query the database.

## API

All endpoints are public (no authentication). Responses are JSON; errors use `{"error": "message"}`.

| Method | Path | Description | Success | Notable errors |
| --- | --- | --- | --- | --- |
| `GET` | `/health` | Liveness check | `200` | — |
| `GET` | `/tickets` | List all tickets, newest first, with latest triage category and priority | `200` | `500` |
| `POST` | `/tickets` | Create a ticket (`title`, `description`); rate limited | `201` | `400`, `413`, `429`, `500` |
| `GET` | `/tickets/{id}` | Get a ticket | `200` | `400`, `404`, `500` |
| `PATCH` | `/tickets/{id}` | Update the status (`open`, `in_progress`, `resolved`) | `204` | `400`, `404`, `413`, `500` |
| `POST` | `/tickets/{id}/triage` | Run an AI triage and store it; rate limited, empty body | `201` | `400`, `404`, `429`, `500`, `503` |
| `GET` | `/tickets/{id}/triages` | Triage history, newest first | `200` | `400`, `404`, `500` |
| `GET` | `/tickets/{id}/triage` | Latest triage | `200` | `400`, `404` (ticket or triage not found), `500` |
| `GET` | `/dashboard/stats` | `open_tickets`, `high_priority`, `ai_analyzed` counters | `200` | `500` |

## Technical decisions

- **Go standard library for HTTP.** Go's `http.ServeMux` supports method and path-parameter patterns, so routing, middleware, and server timeouts are implemented without a web framework, keeping the dependency list short.
- **Layered backend with small interfaces.** Handlers, services, and repositories have separate responsibilities, and each consumer declares the minimal interface it needs. This keeps HTTP, business logic, and SQL apart and makes most code testable without a database or OpenAI.
- **PostgreSQL with plain SQL.** Queries are written by hand with pgx, including `LEFT JOIN LATERAL` to attach each ticket's latest triage and a single-round-trip dashboard query. Constraints in the schema back up the application's validation.
- **Append-only triage history.** Storing every analysis instead of overwriting it keeps a record of how AI suggestions changed across runs and which model produced them.
- **Structured AI output with server-side validation.** A strict JSON schema makes the model's output machine-readable, and the API still validates enums, ranges, and lengths before saving, since a schema alone does not guarantee usable data.
- **Bounded AI usage.** Per-client rate limits, a global quota, a concurrency cap, token limits, timeouts, and limited retries keep a public demo from producing unbounded OpenAI costs.
- **Next.js Server Components for reads.** Dashboard and list data are fetched on the server and streamed with Suspense fallbacks; only interactive parts run as client components.
- **Manual, explicit migrations.** A dedicated golang-migrate image keeps schema changes as deliberate operator actions, separate from application startup and CI.

## Known limitations

- **Portfolio scope**: no authentication, authorization, user accounts, or multi-tenancy. The demo is public and must contain fictional data only.
- **External AI dependency**: triage requires an OpenAI API key, incurs per-request costs, and can return imperfect or inconsistent results.
- **In-memory limits**: rate limits, the global triage quota, and concurrency slots reset on restart and are per process.
- **Client-side list operations**: `GET /tickets` returns every ticket, and search, filtering, sorting, and pagination are done in the browser. This is fine for a demo dataset but would not scale to large volumes.
- **No ticket editing or deletion**: only the status of a ticket can be changed after creation.
- **Cold starts**: the API runs on Render's free tier, so the first request after an idle period can be slow.

## Project status

Tickordo is a **deployed portfolio project** with a working public demo. The core workflow — creating tickets, managing status, running AI triage, reviewing triage history, and viewing dashboard statistics — is implemented and covered by automated tests in CI. It is not intended for production use.

## License

This project is licensed under the [MIT License](LICENSE). Copyright (c) 2026 Piter Gomes.
