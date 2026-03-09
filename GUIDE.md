# HN Jobs — Stack Guide (for Java/Kotlin developers like me)

Every concept is explained by comparing it to its Java or Kotlin equivalent. If you know Java/Kotlin, you already understand most of the ideas here — the syntax is just different.

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Backend — Go](#2-backend--go)
   - [The Go language vs Java/Kotlin](#the-go-language-vs-javakotlin)
   - [go.mod](#gomod)
   - [cmd/server/main.go](#cmdservermainго)
   - [internal/models/job.go](#internalmodelsjobgo)
   - [internal/db/db.go](#internaldbdbgo)
   - [internal/db/jobs.go](#internaldbjobsgo)
   - [internal/scraper/scraper.go](#internalscraperscrapergo)
   - [internal/api/handler.go](#internalapihandlergo)
3. [Frontend — TypeScript + Next.js](#3-frontend--typescript--nextjs)
   - [TypeScript vs Kotlin](#typescript-vs-kotlin)
   - [package.json](#packagejson)
   - [next.config.ts](#nextconfigts)
   - [app/layout.tsx](#applayouttsx)
   - [app/providers.tsx](#appproviderstsx)
   - [app/page.tsx](#apppagetsx)
   - [lib/api.ts](#libapits)
   - [components/FilterBar.tsx](#componentsfilterbartsx)
   - [components/JobCard.tsx](#componentsjobcardtsx)
   - [components/Pagination.tsx](#componentspaginationtsx)
4. [Infrastructure — Docker](#4-infrastructure--docker)
5. [Data Flow — End to End](#5-data-flow--end-to-end)
6. [SQLite + FTS5 Explained](#6-sqlite--fts5-explained)
7. [Quick Reference: Java/Kotlin → Go/TS](#7-quick-reference-javakotlin--gots)

---

## 1. Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│                    Browser                          │
│           Next.js (React) on :3000                  │
│   FilterBar → TanStack Query → JobCard / Pagination │
└────────────────────┬────────────────────────────────┘
                     │ HTTP GET /api/jobs?keywords=...
                     ▼
┌─────────────────────────────────────────────────────┐
│              Go Backend on :8080                    │
│   Chi Router → Handler → Store → SQLite (FTS5)     │
│                                                     │
│   Cron (daily) → Scraper → HN Firebase API         │
│                          → Algolia HN Search API   │
└─────────────────────────────────────────────────────┘
```

Think of it as a Spring Boot app (Go backend) serving a React SPA (Next.js frontend). The backend has two responsibilities:
- **Scraping**: a background job fetches HN job posts and stores them in SQLite (like a `@Scheduled` Spring service)
- **Serving**: a REST controller answers filter queries from the frontend

---

## 2. Backend — Go

### The Go language vs Java/Kotlin

| Concept | Java/Kotlin | Go |
|---|---|---|
| Build file | `build.gradle` / `pom.xml` | `go.mod` |
| Package | `package com.example` (namespace) | `package models` (= the directory name) |
| Access modifier | `private`, `public`, `internal` | Uppercase = exported (public), lowercase = unexported (private) |
| Class | `class` / `data class` | `struct` |
| Interface | `interface Foo { fun bar() }` | `interface Foo { Bar() }` — implicitly implemented |
| Null safety | `String?` (Kotlin) / `Optional<T>` | `*string` (pointer = nullable) |
| Generics | `List<String>` | `[]string` (slice) — generics exist but rarely used |
| Exception handling | `try/catch` / checked exceptions | Functions return `(value, error)` — no exceptions |
| Concurrency | `Thread`, `Coroutine` | `goroutine` (prefixed with `go`) |
| Type inference | `val x = 5` | `x := 5` |
| No-op return type | `void` / `Unit` | no return type listed |
| String formatting | `String.format(...)` / `"$var"` | `fmt.Sprintf("...", var)` |
| Inheritance | `class A : B()` | No inheritance. Use composition. |
| `this` | `this` | Receiver variable (conventionally the first letter of the type) |

**The biggest mental shift**: Go has no exceptions. Every function that can fail returns an error as a second return value. You check it explicitly every time:

```go
// Go
val, err := someFunc()
if err != nil {
    return fmt.Errorf("context: %w", err)  // wrap and return up
}
// use val
```

```kotlin
// Kotlin equivalent (checked exception style)
val val = try {
    someFunc()
} catch (e: Exception) {
    throw RuntimeException("context", e)
}
```

It's verbose but explicit — you can never accidentally swallow an error.

---

### `go.mod`

```
module github.com/lyson/hn-jobs   ← like groupId:artifactId in Maven
go 1.22                           ← minimum Go version

require (
    github.com/go-chi/chi/v5       ← HTTP router (like Spring MVC's DispatcherServlet, but tiny)
    github.com/go-chi/cors         ← CORS middleware
    github.com/PuerkitoBio/goquery ← HTML parser (like Jsoup in Java)
    github.com/mattn/go-sqlite3    ← SQLite JDBC-equivalent driver
    github.com/robfig/cron/v3      ← like Spring's @Scheduled
)
```

`go.mod` = `build.gradle`. After `go mod tidy`, `go.sum` is generated — like Gradle's lock file, it contains hashes of every dependency for reproducible builds.

The module name (`github.com/lyson/hn-jobs`) is used in import paths within the project:
```go
import "github.com/lyson/hn-jobs/internal/models"
// equivalent to:
import com.lyson.hnjobs.models.*;
```

**`internal/` directory**: Go enforces this — packages inside `internal/` can only be imported by code in the parent directory tree. It's Go's version of Java's package-private, but at the directory level.

---

### `cmd/server/main.go`

```
backend/
└── cmd/
    └── server/
        └── main.go   ← program entrypoint
```

`cmd/` is a convention for "runnable programs". Like having multiple `main` classes in separate modules — you could add `cmd/migrator/`, `cmd/worker/`, etc.

```go
func main() {
    // 1. Read env vars (like @Value("${DB_PATH:./data/jobs.db}") in Spring)
    dbPath := getenv("DB_PATH", "./data/jobs.db")
    addr   := getenv("ADDR", ":8080")

    // 2. Open DB — runs CREATE TABLE IF NOT EXISTS migrations
    database, err := db.Open(dbPath)
    if err != nil {
        log.Fatalf("open db: %v", err)  // like System.exit(1) after logging
    }
    defer database.Close()  // ← runs when main() returns (like try-finally)

    // 3. Create Store (Repository layer)
    store := db.NewStore(database)

    // 4. Run scraper in background — like new Thread(sc::run).start()
    sc := scraper.New(store)
    go sc.Run()   // "go" keyword = launch goroutine

    // 5. Schedule daily scraping — like @Scheduled(cron = "0 9 * * *")
    c := cron.New()
    c.AddFunc("0 9 * * *", sc.Run)
    c.Start()
    defer c.Stop()

    // 6. Start HTTP server (blocking, like SpringApplication.run())
    router := api.NewRouter(store)
    http.ListenAndServe(addr, router)
}
```

**`defer`**: schedules a function call to run when the surrounding function returns — regardless of how it returns. Think of it as Go's `try { ... } finally { close() }`. You write it next to the resource acquisition so you don't forget:

```go
f, _ := os.Open("file.txt")
defer f.Close()  // will run when function exits
// ... use f
```

```kotlin
// Kotlin equivalent
File("file.txt").bufferedReader().use { reader ->
    // ... use reader (closed automatically)
}
```

**Goroutines**: much cheaper than threads (~2KB stack vs ~1MB for a JVM thread). The Go runtime multiplexes thousands of goroutines onto a small thread pool automatically. `go fn()` is the entire API — no `ExecutorService`, no `CompletableFuture`.

---

### `internal/models/job.go`

Defines the data shapes used throughout the app. In Kotlin you'd use data classes; in Go you use structs.

```go
// Go
type Job struct {
    ID         int       `json:"id"`
    HNItemID   int       `json:"hn_item_id"`
    Remote     bool      `json:"remote"`
    SalaryMin  *int      `json:"salary_min,omitempty"`
    PostedAt   time.Time `json:"posted_at"`
}
```

```kotlin
// Kotlin equivalent
data class Job(
    @JsonProperty("id")          val id: Int,
    @JsonProperty("hn_item_id")  val hnItemId: Int,
    @JsonProperty("remote")      val remote: Boolean,
    @JsonProperty("salary_min")  val salaryMin: Int?,      // nullable
    @JsonProperty("posted_at")   val postedAt: Instant,
)
```

**Struct tags** (the backtick strings after each field) are Go's version of Jackson annotations:
- `` `json:"id"` `` → `@JsonProperty("id")`
- `` `json:"salary_min,omitempty"` `` → `@JsonInclude(NON_NULL)` + `@JsonProperty("salary_min")`

**Pointers for nullable fields**: Go's primitive types (`int`, `bool`) can't be null. To represent "no value", you use a pointer `*int`. A nil pointer serializes to JSON `null`.

```go
// Go nullable types
SalaryMin *int   // nil = absent, &42 = has value

// Kotlin equivalent
val salaryMin: Int? = null
```

**The `JobFilter` struct** — a DTO / query object:
```go
type JobFilter struct {
    Keywords  string
    Remote    *bool  // nil = no filter, &true = remote only, &false = onsite only
    SalaryMin *int
    Page      int
    PageSize  int
}
```

Using `*bool` instead of `bool` lets us represent three states: unset (nil), true, false. In Kotlin: `Boolean?`.

---

### `internal/db/db.go`

Opens the database connection and runs schema migrations.

```go
func Open(path string) (*sql.DB, error) {
    // Connection string with options (like JDBC URL params)
    db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
    db.SetMaxOpenConns(1)  // SQLite: only 1 concurrent writer
    migrate(db)
    return db, nil
}
```

`*sql.DB` in Go ≈ `DataSource` in Java. It's a connection pool (though here limited to 1 connection for SQLite).

**`_journal_mode=WAL`** (Write-Ahead Logging): SQLite's default mode locks the entire file for writes, blocking reads. WAL mode allows concurrent reads while one write is happening. Equivalent to switching from table-level locking to MVCC in a proper database.

**`_foreign_keys=on`**: SQLite ignores FK constraints by default (for historical compatibility). This enables enforcement. Always include it.

**Migrations** — the `migrate()` function runs raw DDL:
```go
func migrate(db *sql.DB) error {
    _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS threads ( ... );
        CREATE TABLE IF NOT EXISTS jobs ( ... );
        CREATE INDEX IF NOT EXISTS ...;
        CREATE VIRTUAL TABLE IF NOT EXISTS jobs_fts USING fts5(...);
        CREATE TRIGGER IF NOT EXISTS ...;
    `)
    return err
}
```

For a production app you'd use `golang-migrate` (equivalent to Flyway/Liquibase). For this project, `IF NOT EXISTS` makes the migration idempotent — safe to run every startup.

**FTS5 virtual table**:
```sql
CREATE VIRTUAL TABLE IF NOT EXISTS jobs_fts USING fts5(
    text, company, location,
    content=jobs,      -- reads text from the jobs table (no data duplication)
    content_rowid=id   -- jobs.id = the FTS rowid
);
```

Think of it as a database-managed inverted index (like Elasticsearch, but built into SQLite). The `content=jobs` means it's a "shadow table" — it stores index structures but reads actual text from `jobs` when needed.

**Triggers** keep the index in sync automatically — you INSERT into `jobs`, the trigger fires and updates `jobs_fts`. You never touch `jobs_fts` directly in application code.

---

### `internal/db/jobs.go`

The **Store** struct — this is the Repository pattern, identical to what you'd write in Spring Data:

```go
type Store struct {
    db *sql.DB  // unexported = private field
}

func NewStore(db *sql.DB) *Store {  // constructor function (Go has no constructors)
    return &Store{db: db}
}
```

```kotlin
// Kotlin equivalent
class Store(private val db: DataSource) {
    companion object {
        fun create(db: DataSource) = Store(db)
    }
}
```

**Methods on structs** — Go uses "receiver" syntax instead of methods inside a class body:

```go
// Go: method on Store
func (s *Store) UpsertJob(j *models.Job) error { ... }
//    ^^^^^^^^ receiver (like "this" but named)
```

```kotlin
// Kotlin equivalent
fun Store.upsertJob(j: Job): Unit { ... }
// or inside the class:
fun upsertJob(j: Job) { ... }
```

**Upsert with `ON CONFLICT DO UPDATE`**:
```sql
INSERT INTO jobs (hn_item_id, text, ...)
VALUES (?, ?, ...)
ON CONFLICT(hn_item_id) DO UPDATE SET
    text = excluded.text,   -- "excluded" = the row that was rejected
    ...
```

`excluded.` refers to the values you tried to insert but were rejected by the unique constraint. This is standard SQL upsert — equivalent to JPA's `merge()` or Spring's `save()` on an existing entity.

**Dynamic query building in `ListJobs`**:

```go
where := []string{"1=1"}   // always-true base condition
args  := []any{}           // []any ≈ List<Object> in Java

if f.Keywords != "" {
    where = append(where, "j.id IN (SELECT rowid FROM jobs_fts WHERE jobs_fts MATCH ?)")
    args  = append(args, f.Keywords)
    //                            ↑ NEVER concatenate user input — use placeholders
}
if f.Remote != nil {
    if *f.Remote {
        where = append(where, "j.remote = 1")
    }
}

query := fmt.Sprintf(
    "SELECT ... FROM jobs j WHERE %s ORDER BY posted_at DESC LIMIT ? OFFSET ?",
    strings.Join(where, " AND "),
)
rows, err := s.db.Query(query, args...)
```

This is like JPA Criteria API or JOOQ — building a query dynamically based on which filters are set. The `?` placeholders (prepared statement parameters) prevent SQL injection.

**Scanning rows** — Go's JDBC-style row iteration:

```go
rows, _ := db.Query("SELECT id, salary_min, salary_max FROM jobs WHERE ...")
defer rows.Close()

for rows.Next() {
    var id int
    var sMin, sMax sql.NullInt64  // nullable SQL integer
    rows.Scan(&id, &sMin, &sMax)  // & = pointer to variable (pass by reference)

    if sMin.Valid {
        val := int(sMin.Int64)
        job.SalaryMin = &val
    }
}
```

```kotlin
// Kotlin/JDBC equivalent
resultSet.use { rs ->
    while (rs.next()) {
        val id = rs.getInt("id")
        val sMin = rs.getObject("salary_min") as Int?
    }
}
```

`sql.NullInt64` = Java's nullable boxed `Long` from a ResultSet — it has `.Valid` (was it NULL?) and `.Int64` (the actual value).

`&variable` passes a pointer to the variable so `Scan` can write into it. Think of it as passing by reference — like `out` parameters in C#.

---

### `internal/scraper/scraper.go`

The scraper calls two external APIs then stores the results.

**The `Scraper` struct** — like a Spring `@Service`:
```go
type Scraper struct {
    store  *db.Store
    client *http.Client  // reusable HTTP client (like OkHttpClient)
}

func New(store *db.Store) *Scraper {
    return &Scraper{
        store:  store,
        client: &http.Client{Timeout: 30 * time.Second},
    }
}
```

**JSON decoding** — like Jackson `ObjectMapper.readValue()`:
```go
type hnItem struct {
    ID    int    `json:"id"`
    By    string `json:"by"`
    Kids  []int  `json:"kids"`   // []int = List<Integer>
    Text  string `json:"text"`
    Time  int64  `json:"time"`   // Unix timestamp
}

resp, _ := http.Get(url)
defer resp.Body.Close()

var item hnItem
json.NewDecoder(resp.Body).Decode(&item)
// ↑ like objectMapper.readValue(resp.getBody(), HnItem.class)
```

**Goquery** — like Jsoup for Java:
```go
// Java/Jsoup
Document doc = Jsoup.parse("<body>" + html + "</body>");
Elements paragraphs = doc.select("p");

// Go/Goquery
doc, _ := goquery.NewDocumentFromReader(strings.NewReader("<body>" + html + "</body>"))
doc.Find("p").Each(func(_ int, s *goquery.Selection) {
    // s is like a Jsoup Element
    s.ReplaceWithHtml("\n" + s.Text() + "\n")
})
text := doc.Find("body").Text()
```

**Regex** — like Java's `Pattern` + `Matcher`:
```go
// Go — compile once at package level (like static final Pattern in Java)
var salaryRe = regexp.MustCompile(`(?i)\$(\d{2,3})[Kk]?\s*(?:[-–]\s*\$?(\d{2,3})[Kk]?)?`)

// Use it
m := salaryRe.FindStringSubmatch(text)
// m[0] = full match, m[1] = first capture group, m[2] = second
```

```java
// Java equivalent
private static final Pattern SALARY_RE = Pattern.compile(
    "(?i)\\$(\\d{2,3})[Kk]?\\s*(?:[-–]\\s*\\$?(\\d{2,3})[Kk]?)?");

Matcher m = SALARY_RE.matcher(text);
if (m.find()) {
    String group1 = m.group(1);
}
```

`regexp.MustCompile` panics at startup if the pattern is invalid (instead of returning an error). This is intentional — a bad regex is a programming error, not a runtime error. Same as throwing in a static initializer.

**Package-level variables** (`var salaryRe = ...`) are initialized once when the program starts, similar to `static final` fields in Java.

**Rate limiting** between API calls:
```go
time.Sleep(100 * time.Millisecond)
// like Thread.sleep(100) but idiomatic Go uses time.Duration
```

---

### `internal/api/handler.go`

The HTTP layer — think of Chi as a minimalist `DispatcherServlet` without the Spring container magic.

```go
func NewRouter(store *db.Store) http.Handler {
    r := chi.NewRouter()

    // Middleware chain — like Spring's HandlerInterceptor or Servlet Filters
    r.Use(middleware.Logger)    // logs every request (like access log)
    r.Use(middleware.Recoverer) // catches panics → returns 500 (like @ExceptionHandler)
    r.Use(cors.Handler(...))    // sets CORS headers

    r.Get("/api/jobs", h.listJobs)   // like @GetMapping("/api/jobs")
    r.Get("/api/health", h.health)

    return r
}
```

**Handler signature** — Go's equivalent of a Spring controller method:
```go
func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
    // w = HttpServletResponse
    // r = HttpServletRequest
}
```

```kotlin
// Spring equivalent
@GetMapping("/api/jobs")
fun listJobs(@RequestParam params: Map<String, String>): ResponseEntity<JobsResponse>
```

**Reading query parameters**:
```go
q := r.URL.Query()                      // like request.getParameterMap()
keywords := q.Get("keywords")           // empty string if absent

if v := q.Get("remote"); v == "true" {  // if-with-init: declares v, then checks it
    b := true
    f.Remote = &b
}
```

**Writing JSON response** — like `@ResponseBody` with Jackson:
```go
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(resp)
// like objectMapper.writeValue(response.getWriter(), resp)
```

**CORS**: browsers refuse to let JavaScript on `localhost:3000` call APIs on `localhost:8080` unless the server explicitly allows it via `Access-Control-Allow-Origin` headers. The `cors` middleware adds these headers automatically. Without it, every fetch from the frontend would be blocked.

---

## 3. Frontend — TypeScript + Next.js

### TypeScript vs Kotlin

TypeScript is essentially Kotlin for the browser — a typed layer on top of JavaScript, just as Kotlin is a typed layer on top of the JVM.

| Concept | Kotlin | TypeScript |
|---|---|---|
| Data class | `data class Job(val id: Int, ...)` | `interface Job { id: number; ... }` |
| Nullable | `Int?` | `number \| undefined` or `number?` in optional fields |
| Type alias | `typealias JobId = Int` | `type JobId = number` |
| Utility types | — | `Partial<T>`, `Required<T>`, `Pick<T, K>` |
| Null coalescing | `val x = y ?: "default"` | `const x = y ?? "default"` |
| Safe call | `obj?.field` | `obj?.field` (identical) |
| String template | `"Hello $name"` | `` `Hello ${name}` `` |
| Lambda | `{ x -> x + 1 }` | `(x) => x + 1` |
| Coroutine suspend | `suspend fun fetch()` | `async function fetch()` + `await` |
| `Any` | `Any` | `any` (avoid) or `unknown` |

**React components** are like Kotlin `@Composable` functions — functions that return UI, which can call other such functions:

```tsx
// React/TypeScript
function JobCard({ job }: { job: Job }) {
    return <article>...</article>
}
```

```kotlin
// Jetpack Compose equivalent
@Composable
fun JobCard(job: Job) {
    Card { ... }
}
```

**JSX** (the `<article>` syntax inside TypeScript) compiles down to function calls. It's syntactic sugar — not HTML. Attributes become function arguments.

---

### `package.json`

```json
{
  "dependencies": {
    "next": "16.x",                    // the framework
    "@tanstack/react-query": "^5",     // server state library (explained below)
    "lucide-react": "^0.447",          // icon library (like Material Icons)
    "react": "19.x",                   // UI library
    "react-dom": "19.x"                // renders React to the browser DOM
  },
  "devDependencies": {
    "tailwindcss": "^4",               // utility CSS framework
    "typescript": "^5"                 // type checker
  }
}
```

`package.json` = `build.gradle`. `npm install` = `gradle dependencies`. `npm run dev` = `./gradlew bootRun`.

The key dependency to understand is **TanStack Query**: it manages the lifecycle of server data (loading, caching, error states, refetching). Without it you'd write `useEffect` hooks manually — like writing `JdbcTemplate` by hand instead of using Spring Data.

---

### `next.config.ts`

```ts
const nextConfig: NextConfig = {
  output: "standalone",
};
```

`output: "standalone"` tells Next.js to bundle all required files into `.next/standalone/` — a self-contained directory with a single `server.js` entry point. This is what the Docker image runs.

---

### `app/layout.tsx`

In Next.js, the `app/` directory structure defines routes. `layout.tsx` wraps every page — like a base Activity/Fragment in Android, or a master template in Spring MVC.

```tsx
// layout.tsx — runs on the SERVER by default
export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className="bg-zinc-950 text-zinc-100 min-h-screen">
        <Providers>{children}</Providers>
        {/* children = the current page component */}
      </body>
    </html>
  );
}
```

`children` is the content slot — like `{children}` in React = `@yield` in templates = `slot` in web components. The layout renders once; navigating between pages only re-renders the page content, not the layout.

**Server Components** (the default): run only on the server, like a Thymeleaf template. They can't use browser APIs, React hooks, or event listeners. Faster — no JavaScript sent to the browser for them.

---

### `app/providers.tsx`

```tsx
"use client";  // ← opt-in to browser execution

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(() => new QueryClient({
    defaultOptions: { queries: { staleTime: 60_000 } }
  }));

  return (
    <QueryClientProvider client={queryClient}>
      {children}
    </QueryClientProvider>
  );
}
```

**`"use client"`** directive: marks this as a **Client Component** — it runs in the browser and can use hooks (`useState`, `useEffect`) and browser APIs. Without it, React hooks throw an error because they need browser state.

`QueryClientProvider` is a React Context provider — it makes the `QueryClient` accessible to all child components without passing it as props. Like a Spring `ApplicationContext` scoped to the component tree.

**Why a separate file?** `layout.tsx` is a Server Component. Importing a Client Component from a Server Component is fine — the server component renders the tree and the client component takes over in the browser. But we can't make `layout.tsx` itself a Client Component (it would lose server rendering benefits).

**`useState(() => new QueryClient())`**: the arrow function form (lazy initializer) means `new QueryClient()` only runs once — not on every render. Identical to Kotlin lazy: `val client by lazy { QueryClient() }`.

---

### `app/page.tsx`

The main page component. Manages all filter state and orchestrates data fetching.

```tsx
"use client";

const DEFAULT_FILTERS: JobFilters = { page: 1, page_size: 20 };

export default function Home() {
  // State = like a ViewModel's MutableStateFlow<JobFilters>
  const [filters, setFilters] = useState<JobFilters>(DEFAULT_FILTERS);

  // TanStack Query: like calling a suspend fun from a Composable
  const { data, isLoading, isError, isFetching } = useQuery({
    queryKey: ["jobs", filters],   // cache key — like a cache map key
    queryFn: () => fetchJobs(filters),  // the suspend-like async function
  });

  return (
    <div>
      <FilterBar filters={filters} onChange={setFilters} />

      {isLoading && <Loader />}   // ← conditional rendering: like `if (loading) { ... }`
      {isError && <ErrorMsg />}

      {data?.jobs?.map(job =>     // optional chaining: data?.jobs = data?.jobs ?: emptyList()
        <JobCard key={job.id} job={job} />  // key = like RecyclerView item ID
      )}

      <Pagination
        page={data?.page ?? 1}
        totalPages={data?.total_pages ?? 1}
        onPage={p => setFilters(f => ({ ...f, page: p }))}
      />
    </div>
  );
}
```

**`useQuery`** — the core TanStack Query hook:

| Property | Meaning | Spring/Kotlin analogy |
|---|---|---|
| `queryKey` | Cache identifier | Map key for a result cache |
| `queryFn` | Async data fetcher | `suspend fun` called by coroutine |
| `isLoading` | First load, no cache | — |
| `isFetching` | Any in-flight request | `isActive` on a Job |
| `isError` | `queryFn` threw | Caught exception |
| `data` | The fetched value | Coroutine result |

When `filters` changes, `queryKey` changes, TanStack Query detects the new key and calls `queryFn` again. Previous results for the same key are served instantly from cache.

**`{...f, page: p}`** — object spread. Like Kotlin `copy()`:
```ts
{ ...f, page: p }
// equals:
f.copy(page = p)  // Kotlin data class
```

---

### `lib/api.ts`

The API client layer — like a Retrofit/OkHttp service interface in Android:

```ts
const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
//                                          ↑ only NEXT_PUBLIC_ vars are exposed to browser
```

```ts
export async function fetchJobs(filters: JobFilters): Promise<JobsResponse> {
  const params = new URLSearchParams();
  if (filters.keywords) params.set("keywords", filters.keywords);
  if (filters.remote !== undefined) params.set("remote", String(filters.remote));
  // ...

  const res = await fetch(`${API_BASE}/api/jobs?${params}`);
  //          ↑ like OkHttpClient.newCall(request).execute()
  if (!res.ok) throw new Error("Failed to fetch jobs");
  return res.json();  // deserialize JSON → typed object
}
```

`URLSearchParams` safely encodes query strings (handles spaces, `&`, `=` in values). Like `UriBuilder` in Spring, or OkHttp's `HttpUrl.Builder`.

`throw new Error(...)` on HTTP errors is important — TanStack Query catches thrown errors and sets `isError = true`. If you don't throw, a 500 response looks like success.

**Utility functions**:
```ts
export function formatSalary(min?: number, max?: number): string {
  const fmt = (n: number) => n >= 1000 ? `$${Math.round(n / 1000)}K` : `$${n}`;
  if (min && max) return `${fmt(min)} – ${fmt(max)}`;
  if (min) return `${fmt(min)}+`;
  return fmt(max!);  // ! = non-null assertion (like !! in Kotlin)
}

export function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const days = Math.floor(diff / 86_400_000);
  if (days === 0) return "today";
  if (days < 30) return `${days}d ago`;
  return `${Math.floor(days / 30)}mo ago`;
}
```

---

### `components/FilterBar.tsx`

The sticky filter bar — demonstrates the key React state management patterns.

```tsx
"use client";

interface Props {
  filters: JobFilters;     // current filter state (owned by parent — page.tsx)
  onChange: (f: JobFilters) => void;  // callback to update parent state
}
```

This is **"lifted state"** — the filter state lives in `page.tsx` and flows down via props. `FilterBar` calls `onChange` to request a state update. Like the Observer pattern, or like a Compose `onValueChange` lambda.

```tsx
export function FilterBar({ filters, onChange }: Props) {
  // Local state: tracks what's typed, before submitting to parent
  const [keywordsInput, setKeywordsInput] = useState(filters.keywords ?? "");

  // Memoized function — like caching a lambda reference
  const submit = useCallback((partial: Partial<JobFilters>) => {
    onChange({ ...filters, ...partial, page: 1 });  // always reset to page 1
  }, [filters, onChange]);
  //  ↑ dependency array: recreate submit only when these change
```

**Two-tier state** for the text inputs: `keywordsInput` is what's in the box right now. `filters.keywords` is what's been submitted to the API. They sync on `Enter` or when the field loses focus — not on every keystroke (avoids a network request per character).

**`Partial<JobFilters>`** — TypeScript utility type. Makes every field optional:
```ts
Partial<{ keywords: string; page: number }>
// becomes:
{ keywords?: string; page?: number }
```

Like a Kotlin data class where all fields have defaults, or a Builder pattern.

```tsx
  return (
    <div className="sticky top-0 z-10 bg-zinc-900/95 backdrop-blur ...">
```

**Tailwind CSS**: every visual property is a utility class. Instead of writing a `.filter-bar { position: sticky; top: 0; ... }` stylesheet, you compose classes inline:
- `sticky` → `position: sticky`
- `top-0` → `top: 0`
- `z-10` → `z-index: 10`
- `bg-zinc-900/95` → `background-color: zinc-900 at 95% opacity`
- `backdrop-blur` → CSS blur on the background behind the element

No class name collisions, no specificity wars, no separate CSS files.

---

### `components/JobCard.tsx`

```tsx
export function JobCard({ job }: Props) {
  const [expanded, setExpanded] = useState(false);  // local toggle state

  const lines = job.text.split("\n");
  const preview = lines.slice(0, 3).join(" ").slice(0, 200);
  const hasMore = job.text.length > 200 || lines.length > 3;

  return (
    <article className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 ...">

      {/* Salary badge — only renders if salary exists */}
      {salary && (
        <span className="text-emerald-400">
          <DollarSign className="w-3.5 h-3.5" />  {/* Lucide icon component */}
          {salary}
        </span>
      )}

      {/* Conditional text rendering */}
      {expanded
        ? <pre className="whitespace-pre-wrap font-sans">{job.text}</pre>
        : <p>{preview}…</p>
      }

      {/* Toggle button — only shown if text is long */}
      {hasMore && (
        <button onClick={() => setExpanded(v => !v)}>
          {expanded ? "Show less" : "Show more"}
        </button>
      )}
    </article>
  );
}
```

**`{condition && <Component />}`** — conditional rendering. If `condition` is falsy, nothing renders. Like `if (condition) View(...)` in Compose.

**`setExpanded(v => !v)`** — functional update form. Uses the current value to compute the next value. Safe when the update depends on the current state (avoids stale closure bugs). Like `AtomicBoolean.getAndNegate()`.

**Lucide icons** are React components — just import and use like any other component. `<DollarSign className="w-3.5 h-3.5" />` renders an SVG icon sized via Tailwind.

---

### `components/Pagination.tsx`

Builds a compact page list with ellipsis (1 ... 3 4 **5** 6 7 ... 20):

```tsx
const pages: (number | "...")[] = [];
const delta = 2;  // show 2 pages each side of current page

for (let i = 1; i <= totalPages; i++) {
  if (
    i === 1 ||                            // always show first
    i === totalPages ||                   // always show last
    (i >= page - delta && i < page + delta + 1)  // window around current
  ) {
    pages.push(i);
  } else if (pages[pages.length - 1] !== "...") {
    pages.push("...");  // add ellipsis only once per gap
  }
}
```

`(number | "...")[]` is a TypeScript union type array — like `List<Either<Int, String>>` but simpler. The array can hold either numbers or the string `"..."`.

---

## 4. Infrastructure — Docker

### `backend/Dockerfile` — multi-stage build

Multi-stage builds are like Maven's `package` phase vs the final deployable JAR:

```dockerfile
# Stage 1: compile (like running "go build" in CI)
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache gcc musl-dev  # gcc needed for sqlite3 (CGo = JNI equivalent)

WORKDIR /app
COPY go.mod go.sum ./        # copy dependency manifests first
RUN go mod download           # download deps → cached as a Docker layer
                              # (like Gradle's --build-cache: only re-runs if go.mod changed)
COPY . .
RUN CGO_ENABLED=1 go build -o /server ./cmd/server

# Stage 2: runtime image (like a FROM scratch + just the JAR)
FROM alpine:3.19
RUN apk add --no-cache ca-certificates  # needed for HTTPS (like importing JVM truststore)
COPY --from=builder /server .           # copy only the compiled binary

EXPOSE 8080
CMD ["./server"]
```

The final image is ~15MB vs ~1GB for the builder image. No Go toolchain, no source code, no headers — just the binary and Alpine's minimal OS.

**CGo**: `go-sqlite3` wraps a C library (SQLite). CGo is Go's JNI — it lets Go call C code. This requires `gcc` at compile time. The compiled binary is statically linked, so the runtime image doesn't need gcc.

**Layer caching**: Docker caches each `RUN`/`COPY` instruction. If `go.mod` hasn't changed, `go mod download` is a cache hit — builds are instant. This is why dependencies are downloaded before copying source code.

### `frontend/Dockerfile`

```dockerfile
FROM node:20-alpine AS deps
COPY package.json package-lock.json ./
RUN npm ci  # "clean install" — uses lockfile exactly, like "gradle --locked"

FROM node:20-alpine AS builder
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build   # produces .next/standalone/

FROM node:20-alpine AS runner
COPY --from=builder /app/.next/standalone ./  # self-contained server
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
CMD ["node", "server.js"]
```

### `docker-compose.yml`

```yaml
services:
  backend:
    build: ./backend
    ports:
      - "8080:8080"        # host_port:container_port
    volumes:
      - db_data:/data      # named volume: persists SQLite across restarts
    environment:
      - DB_PATH=/data/jobs.db

  frontend:
    build: ./frontend
    ports:
      - "3000:3000"
    environment:
      # "backend" resolves to the backend container — Docker's built-in DNS
      - NEXT_PUBLIC_API_URL=http://backend:8080
    depends_on:
      - backend  # start backend first (like @DependsOn in Spring)

volumes:
  db_data:  # Docker-managed volume (persists on the host at /var/lib/docker/volumes/)
```

Containers in the same Compose file share a virtual network. `http://backend:8080` resolves by service name — like Kubernetes service DNS, or Spring Cloud's service discovery, but built in.

---

## 5. Data Flow — End to End

### Scraping pipeline

```
Startup
  │
  ├─ go sc.Run()  ──────────────────────────────────────── goroutine
  │                                                           │
  │  sc.findHiringThreads()                                   │
  │    → GET algolia API → [threadID1, threadID2, ...]        │
  │                                                           │
  │  for each threadID:                                       │
  │    sc.scrapeThread(threadID)                              │
  │      → GET /item/{threadID}.json  → hnItem{kids:[...]}   │
  │      → store.UpsertThread(...)    → INSERT INTO threads   │
  │      for each kidID in kids:                              │
  │        → GET /item/{kidID}.json   → hnItem{text: "..."}  │
  │        → parseJob(item)           → extract company,      │
  │                                     location, salary      │
  │        → store.UpsertJob(job)     → INSERT INTO jobs      │
  │                                     (trigger → jobs_fts)  │
  │
  └─ cron schedules sc.Run() again at 9am daily
```

### Request pipeline (user changes a filter)

```
Browser
  │
  ├─ setFilters({ keywords: "rust" })   [useState update → re-render]
  │
  ├─ useQuery detects new queryKey      [TanStack Query]
  │    → calls fetchJobs({ keywords: "rust", page: 1 })
  │    → GET http://localhost:8080/api/jobs?keywords=rust&page=1
  │
Backend
  │
  ├─ Chi router → h.listJobs(w, r)
  │    → parse query params → JobFilter{Keywords: "rust", Page: 1}
  │    → store.ListJobs(filter)
  │         → SELECT COUNT(*) FROM jobs WHERE id IN
  │             (SELECT rowid FROM jobs_fts WHERE jobs_fts MATCH 'rust')
  │         → SELECT j.* FROM jobs j WHERE ... LIMIT 20 OFFSET 0
  │    → json.Encode(JobsResponse{Jobs: [...], Total: 342, ...})
  │
Browser
  │
  └─ TanStack Query receives response
       → updates data state
       → React re-renders: JobCard list + Pagination
```

---

## 6. SQLite + FTS5 Explained

### Why SQLite instead of PostgreSQL?

For this app, SQLite is ideal:
- One writer (the scraper), many readers (API requests)
- No separate process to manage — just a file
- FTS5 is built in — no Elasticsearch needed
- WAL mode handles concurrent reads fine

In WAL mode, SQLite is closer to H2 (embedded, file-based) than to MySQL. It's not suitable for multiple writers from different machines.

### How FTS5 (Full-Text Search) works

FTS5 builds an **inverted index** over text columns — the same data structure used by Elasticsearch/Lucene, but embedded in SQLite.

An inverted index maps **word → list of documents containing that word**:
```
"golang"  → [job_id: 5, job_id: 12, job_id: 89]
"remote"  → [job_id: 1, job_id: 5, job_id: 43, ...]
"stripe"  → [job_id: 12, job_id: 67]
```

A `MATCH 'golang'` query looks up "golang" in this index and returns the row IDs in microseconds — no full table scan.

### FTS5 MATCH syntax

```sql
-- Single word
SELECT rowid FROM jobs_fts WHERE jobs_fts MATCH 'rust';

-- Multiple words (implicit AND — both must appear)
SELECT rowid FROM jobs_fts WHERE jobs_fts MATCH 'rust backend';

-- Exact phrase
SELECT rowid FROM jobs_fts WHERE jobs_fts MATCH '"machine learning"';

-- Column-specific search
SELECT rowid FROM jobs_fts WHERE jobs_fts MATCH 'company:stripe';
```

### The content table trick

```sql
CREATE VIRTUAL TABLE jobs_fts USING fts5(
    text, company, location,
    content=jobs,      -- don't store text, read from jobs table
    content_rowid=id   -- jobs.id = fts rowid
);
```

Without `content=jobs`, FTS5 would store a duplicate copy of all text. The `content=` option makes it a **shadow table** — it stores only the index, reads text from `jobs` when needed. The triggers keep the index in sync:

```sql
-- When a job is inserted, index it for FTS
CREATE TRIGGER jobs_ai AFTER INSERT ON jobs BEGIN
  INSERT INTO jobs_fts(rowid, text, company, location)
  VALUES (new.id, new.text, new.company, new.location);
END;
```

Usage in the API:
```sql
-- Find jobs matching "golang", then get full job data
SELECT j.*
FROM jobs j
WHERE j.id IN (SELECT rowid FROM jobs_fts WHERE jobs_fts MATCH 'golang')
ORDER BY j.posted_at DESC
LIMIT 20 OFFSET 0;
```

---

## 7. Quick Reference: Java/Kotlin → Go/TS

### Go

| Java/Kotlin | Go | Notes |
|---|---|---|
| `build.gradle` / `pom.xml` | `go.mod` | Dependency manifest |
| `class Foo { }` | `type Foo struct { }` | No inheritance in Go |
| `data class` | `struct` + struct tags | Tags control JSON serialization |
| `@JsonProperty("x")` | `` `json:"x"` `` | Backtick string after field |
| `@JsonInclude(NON_NULL)` | `` `json:",omitempty"` `` | Skip field if nil/zero |
| `String?` / `Optional<T>` | `*string` (pointer) | nil pointer = absent |
| `fun foo(): Pair<A, B>` | `func foo() (A, error)` | Errors as return values |
| `try/catch` | `if err != nil` | No exceptions |
| `Thread` / `Coroutine` | `go func()` | Goroutine = green thread |
| `try { } finally { }` | `defer f.Close()` | Runs on function return |
| `val x = 5` (Kotlin) | `x := 5` | Type inference |
| `List<String>` | `[]string` | Slice (dynamic array) |
| `Map<K, V>` | `map[K]V` | Built-in map |
| `Object` / `Any` | `any` | Empty interface |
| `static final Pattern` | `var re = regexp.MustCompile(...)` | Package-level compiled regex |
| `@Service` class | `struct` + constructor function | No DI framework needed |
| `@GetMapping("/path")` | `r.Get("/path", handler)` | Chi router |
| `@Scheduled(cron=...)` | `c.AddFunc("...", fn)` | robfig/cron |
| `DataSource` | `*sql.DB` | Connection pool |
| `JdbcTemplate.query()` | `db.Query(sql, args...)` | Returns `*sql.Rows` |
| `rs.next()` / `rs.getInt()` | `rows.Next()` / `rows.Scan(&v)` | Same concept |
| `Optional<Long>` | `sql.NullInt64` | Nullable DB value |
| `UriBuilder` | `URLSearchParams` | Safe URL building |
| `ObjectMapper.readValue()` | `json.NewDecoder(r).Decode(&v)` | JSON deserialization |
| `Jsoup.parse(html)` | `goquery.NewDocumentFromReader(r)` | HTML parsing |

### TypeScript / React

| Kotlin/Android | TypeScript/React | Notes |
|---|---|---|
| `data class Foo(val x: Int)` | `interface Foo { x: number }` | Type-only, no runtime object |
| `Int?` | `number \| undefined` | Optional field: `x?: number` |
| `x ?: default` | `x ?? default` | Null coalescing (identical concept) |
| `obj?.field` | `obj?.field` | Safe call (identical syntax) |
| `listOf(1, 2, 3)` | `[1, 2, 3]` | Array literal |
| `copy(field = value)` | `{ ...obj, field: value }` | Object spread / immutable update |
| `Foo<T>` | `Foo<T>` | Generics (same syntax) |
| `@Composable fun Card(job: Job)` | `function Card({ job }: { job: Job })` | UI component function |
| `MutableStateFlow<T>` | `useState<T>()` | Reactive state |
| `LaunchedEffect { fetch() }` | `useQuery({ queryFn: fetch })` | Async side effect |
| `ViewModel.uiState.collect {}` | `const { data } = useQuery(...)` | Observing async data |
| `if (loading) CircularProgress()` | `{isLoading && <Spinner />}` | Conditional render |
| `items.forEach { Card(it) }` | `items.map(item => <Card key={item.id} item={item} />)` | List render |
| `suspend fun fetch(): T` | `async function fetch(): Promise<T>` | Async function |
| `coroutineScope.launch { }` | `async/await` inside `queryFn` | Async execution |
