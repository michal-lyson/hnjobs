# HN Jobs

Browse and search Hacker News "Who is Hiring?" posts with filters for keywords, location, remote region, salary, and date. Includes a job trends page showing the most mentioned technologies over time.

## Stack

- **Backend**: Go, Chi, Goquery, SQLite + FTS5, robfig/cron
- **Frontend**: Next.js, Tailwind CSS, Lucide icons, TanStack Query
- **Infrastructure**: Docker, docker-compose

## Quick Start

### With Docker (recommended)

```bash
docker compose up --build
```

- Frontend: http://localhost:3000
- Backend API: http://localhost:8080

To wipe the database and re-scrape from scratch:

```bash
docker compose down -v && docker compose up --build
```

> Note: a full scrape covers ~36 threads with hundreds of comments each. Expect 15–30 minutes to complete. The API is usable immediately — data appears as scraping progresses.

### Without Docker

**Backend**

```bash
cd backend
go mod download
DB_PATH=./data/jobs.db go run ./cmd/server
```

Requires Go 1.22+ and `gcc` (for sqlite3 CGo).

**Frontend**

```bash
cd frontend
npm install
npm run dev
```

Requires Node 20+.

## Environment Variables

### Backend

| Variable | Default | Description |
|---|---|---|
| `DB_PATH` | `./data/jobs.db` | SQLite database file path |
| `ADDR` | `:8080` | HTTP listen address |

### Frontend

| Variable | Default | Description |
|---|---|---|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | Backend API base URL |

Set in `frontend/.env.local` for local development.

## API

### `GET /api/jobs`

Returns paginated job listings with optional filters.

**Query parameters**

| Parameter | Type | Example | Description |
|---|---|---|---|
| `keywords` | string | `react typescript` | Full-text search, comma/space separated, results sorted by date |
| `location` | string | `Berlin` | Location substring match |
| `remote_region` | string | `eu` | `any`, `us`, `eu`, or `global` |
| `salary_min` | int | `100000` | Minimum salary (currency-agnostic) |
| `date_from` | date | `2024-01-01` | Posted on or after this date |
| `page` | int | `2` | Page number (default: 1) |
| `page_size` | int | `20` | Results per page (max: 100) |

**Response**

```json
{
  "jobs": [
    {
      "id": 1,
      "hn_item_id": 42069789,
      "author": "throwaway123",
      "company": "Acme Corp",
      "text": "Acme Corp | Software Engineer | Remote (EU) | €90K-€120K...",
      "location": "Remote, Berlin",
      "remote_region": "eu",
      "salary_min": 90000,
      "salary_max": 120000,
      "salary_currency": "EUR",
      "posted_at": "2024-11-01T10:23:00Z",
      "url": "https://news.ycombinator.com/item?id=42069789"
    }
  ],
  "total": 842,
  "page": 1,
  "page_size": 20,
  "total_pages": 43
}
```

### `GET /api/trends`

Returns the 20 most mentioned technology keywords across all scraped threads, with monthly counts.

**Response**

```json
{
  "trends": [
    {
      "keyword": "Python",
      "total": 1240,
      "points": [
        { "month": "January 2025", "count": 312 },
        { "month": "February 2025", "count": 289 }
      ]
    }
  ],
  "months": ["January 2025", "February 2025"]
}
```

### `GET /api/health`

```json
{ "status": "ok" }
```

## How the Scraper Works

On startup (and daily at 9am), the scraper:

1. Queries the Algolia HN API to find the 36 most recent "Who is Hiring?" threads (~3 years)
2. Skips threads older than 45 days that were already fully scraped — only re-scrapes recent threads
3. Fetches each thread's top-level comments via the HN Firebase API
4. Parses each comment with regex to extract company, location, remote region, and salary (multi-currency)
5. Upserts into SQLite (safe to run multiple times — no duplicates)

Salary extraction supports: USD, EUR, GBP, JPY, CHF, CAD, AUD, SGD, INR, SEK, DKK, NOK, PLN, CZK.

Remote region is classified as `us`, `eu`, or `global` based on text patterns near the word "remote". Unspecified remote posts default to `global`.

## Project Structure

```
HN/
├── backend/
│   ├── cmd/server/main.go          Entrypoint, wires everything together
│   ├── internal/
│   │   ├── models/job.go           Shared data types (Job, Thread, Trend, filters)
│   │   ├── db/db.go                Database open + schema migrations
│   │   ├── db/jobs.go              Store: ListJobs, GetTrends, UpsertJob/Thread
│   │   ├── api/handler.go          HTTP handlers: /api/jobs, /api/trends, /api/health
│   │   └── scraper/scraper.go      HN scraping, parsing, multi-currency salary extraction
│   └── Dockerfile
├── frontend/
│   ├── app/
│   │   ├── layout.tsx              Root layout with Providers
│   │   ├── page.tsx                Jobs page (filters + job list)
│   │   ├── trends/page.tsx         Trends page (SVG line chart + bar chart)
│   │   ├── about/page.tsx          About page
│   │   └── providers.tsx           TanStack Query setup
│   ├── components/
│   │   ├── Nav.tsx                 Shared navigation header
│   │   ├── FilterBar.tsx           Sticky filter controls with debounced keyword search
│   │   ├── JobCard.tsx             Expandable job card with salary/location/remote badges
│   │   └── Pagination.tsx          Page navigation
│   ├── lib/api.ts                  API client, fetchJobs(), fetchTrends(), formatSalary()
│   └── Dockerfile
├── docker-compose.yml
├── README.md
└── CLAUDE.md
```
