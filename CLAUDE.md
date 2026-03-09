# CLAUDE.md — HN Jobs

Context for Claude Code when working in this repository.

## Project

A web app that aggregates Hacker News "Who is Hiring?" job posts into a searchable, filterable UI with trends analysis.

## Stack

- **Backend**: Go 1.22, Chi v5, Goquery, SQLite + FTS5, robfig/cron
- **Frontend**: Next.js 16, React 19, Tailwind CSS v4, Lucide icons, TanStack Query v5
- **DB driver**: `go-sqlite3` (CGo — requires `gcc`)

## Commands

### Backend

```bash
cd backend

# Run locally
DB_PATH=./data/jobs.db go run ./cmd/server

# Build binary (must include fts5 tag for SQLite FTS5 support)
CGO_ENABLED=1 go build -tags "fts5" -o server ./cmd/server

# Add/tidy dependencies
go mod tidy

# Type check only
go vet ./...
```

Go binary is at `/usr/local/go/bin/go` — not in PATH by default, use full path or `PATH=$PATH:/usr/local/go/bin`.

### Frontend

```bash
cd frontend

# Install deps (use temp cache if npm cache has permission issues)
npm install --cache /tmp/npm-cache

# Dev server
npm run dev

# Type check
npx tsc --noEmit

# Build
npm run build
```

### Docker

```bash
# Build and start everything
docker compose up --build

# Just backend
docker compose up backend

# Rebuild a single service
docker compose up --build frontend

# Wipe database and re-scrape from scratch
docker compose down -v && docker compose up --build
```

## Architecture

```
Algolia HN API ──► scraper.go ──► SQLite (FTS5)
HN Firebase API ──►           ──►
                                    ▼
Browser ──► Next.js (3000) ──► Go Chi API (8080) ──► SQLite
```

## Key Files

| File | Purpose |
|---|---|
| `backend/cmd/server/main.go` | Entrypoint. Wires DB, store, scraper, cron, HTTP server |
| `backend/internal/db/db.go` | Opens SQLite, runs migrations, defines FTS5 table + triggers |
| `backend/internal/db/jobs.go` | Store: UpsertThread, UpsertJob, ListJobs, GetTrends |
| `backend/internal/scraper/scraper.go` | Fetches HN threads+comments, parses text, multi-currency salary |
| `backend/internal/api/handler.go` | Chi router, CORS, /api/jobs, /api/trends, /api/health |
| `backend/internal/models/job.go` | Job, Thread, JobFilter, JobsResponse, TrendEntry structs |
| `frontend/app/page.tsx` | Jobs page — manages filter state, calls useQuery |
| `frontend/app/trends/page.tsx` | Trends page — SVG line chart + bar chart |
| `frontend/app/about/page.tsx` | About page (static) |
| `frontend/app/providers.tsx` | TanStack QueryClientProvider (must be "use client") |
| `frontend/components/Nav.tsx` | Shared nav header with Jobs / Trends / About links |
| `frontend/components/FilterBar.tsx` | Sticky filter bar — debounced keyword input, remote region dropdown |
| `frontend/components/JobCard.tsx` | Expandable job card with salary/location/remote badges |
| `frontend/lib/api.ts` | fetchJobs(), fetchTrends(), formatSalary(), timeAgo() |

## Database Schema

```sql
threads (id, hn_item_id UNIQUE, title, month, scraped_at, created_at)
jobs    (id, hn_item_id UNIQUE, thread_id FK, author, text, company,
         location, remote_region, salary_min, salary_max, salary_curr, posted_at, url)
jobs_fts  -- FTS5 virtual table over jobs(text, company, location)
```

FTS5 triggers auto-sync on INSERT/UPDATE/DELETE on jobs.

`remote_region` values: `"us"`, `"eu"`, `"global"`, `""` (not remote).

## API

| Endpoint | Description |
|---|---|
| `GET /api/jobs` | Paginated job listings with filters |
| `GET /api/trends` | Top 20 tech keywords with monthly counts |
| `GET /api/health` | Health check |

`GET /api/jobs` params: `keywords` (space/comma separated, BM25 ranked), `location`, `remote_region` (any/us/eu/global), `salary_min`, `date_from` (YYYY-MM-DD), `page`, `page_size`

## Conventions

- Go: errors returned as values, never panicked in handlers. Handlers write JSON directly.
- SQL: always use `?` placeholders — never string-concatenate user input.
- Dynamic query: `where := []string{"1=1"}` pattern in `ListJobs`.
- Results always sort by `posted_at DESC`. FTS5 is used for filtering only, not ranking.
- React: all interactive components have `"use client"` directive at top.
- Tailwind: dark theme using zinc palette (`zinc-900` bg, `zinc-100` text, `orange-500` accents).
- Env vars: `NEXT_PUBLIC_` prefix required for browser-visible vars in Next.js.

## Known Issues / Notes

- `go-sqlite3` requires CGo. `CGO_ENABLED=1` and `-tags "fts5"` must be set for builds; Docker `alpine` image needs `gcc musl-dev`.
- FTS5 input is sanitized — special chars (`"`, `(`, `)`, `*`, `^`, etc.) stripped, reserved words (`AND`, `OR`, `NOT`) filtered before query.
- npm cache may have root-owned files. Use `--cache /tmp/npm-cache` as workaround.
- SQLite `SetMaxOpenConns(1)` is intentional — prevents write lock errors.
- The scraper fetches 36 threads (~3 years). Full scrape takes 15–30 min due to `time.Sleep` between requests to be polite to HN APIs.
- Threads older than 45 days with `scraped_at` set are skipped on subsequent runs. Recent threads are always re-scraped to pick up new comments.
- Salary and remote region extraction are regex-based and best-effort.
- "Go" keyword in trends uses a case-sensitive pattern (`\bGo\b|\bgolang\b`) to avoid matching the verb "go".
- Algolia API URL params must be URL-encoded (use `url.Values`, not raw string interpolation).
