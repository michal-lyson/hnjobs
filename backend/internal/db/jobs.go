package db

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lyson/hn-jobs/internal/models"
)

type Store struct {
	db *sql.DB

	trendsMu      sync.Mutex
	trendsCache   *models.TrendsResponse
	trendsCachedAt time.Time
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// UpsertThread inserts or updates a thread record. Returns the DB id and
// whether the thread has already been fully scraped (scraped_at IS NOT NULL).
func (s *Store) UpsertThread(hnItemID int, title, month string) (int64, bool, error) {
	_, err := s.db.Exec(`
		INSERT INTO threads (hn_item_id, title, month)
		VALUES (?, ?, ?)
		ON CONFLICT(hn_item_id) DO UPDATE SET title=excluded.title
	`, hnItemID, title, month)
	if err != nil {
		return 0, false, err
	}

	var id int64
	var scrapedAt sql.NullTime
	err = s.db.QueryRow(`SELECT id, scraped_at FROM threads WHERE hn_item_id=?`, hnItemID).Scan(&id, &scrapedAt)
	if err != nil {
		return 0, false, err
	}
	return id, scrapedAt.Valid, nil
}

// MarkThreadScraped sets scraped_at to now for the given thread DB id.
func (s *Store) MarkThreadScraped(id int64) error {
	_, err := s.db.Exec(`UPDATE threads SET scraped_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	return err
}

func (s *Store) UpsertJob(j *models.Job) error {
	_, err := s.db.Exec(`
		INSERT INTO jobs (hn_item_id, thread_id, author, text, company, location, remote_region, salary_min, salary_max, salary_curr, posted_at, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(hn_item_id) DO UPDATE SET
			text=excluded.text,
			company=excluded.company,
			location=excluded.location,
			remote_region=excluded.remote_region,
			salary_min=excluded.salary_min,
			salary_max=excluded.salary_max,
			salary_curr=excluded.salary_curr
	`,
		j.HNItemID, j.ThreadID, j.Author, j.Text, j.Company,
		j.Location, j.RemoteRegion, j.SalaryMin, j.SalaryMax, j.SalaryCurr,
		j.PostedAt, j.URL,
	)
	return err
}

// ftsUnsafe matches characters that are special in FTS5 queries.
// We strip them to prevent syntax errors from user input.
// Kept: letters, digits, dot (Next.js), hash (C#), plus (C++), underscore.
var ftsUnsafe = regexp.MustCompile(`[^a-zA-Z0-9_.#+]`)

// ftsReserved are FTS5 boolean operators that must not appear as bare tokens.
var ftsReserved = map[string]bool{"AND": true, "OR": true, "NOT": true}

// buildFTSQuery tokenizes a keyword string (comma/space/semicolon separated)
// into an FTS5 OR query so that posts matching more terms rank higher via bm25.
func buildFTSQuery(keywords string) string {
	tokens := strings.FieldsFunc(keywords, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t'
	})
	var valid []string
	for _, t := range tokens {
		t = ftsUnsafe.ReplaceAllString(t, "")
		if t == "" || ftsReserved[strings.ToUpper(t)] {
			continue
		}
		valid = append(valid, t)
	}
	if len(valid) == 0 {
		return ""
	}
	return strings.Join(valid, " OR ")
}

func (s *Store) ListJobs(f models.JobFilter) (models.JobsResponse, error) {
	if f.PageSize <= 0 {
		f.PageSize = 20
	}
	if f.Page <= 0 {
		f.Page = 1
	}

	ftsQuery := buildFTSQuery(f.Keywords)
	useKeywords := ftsQuery != ""

	// Extra filters on j columns (location, remote, salary, date)
	where := []string{"1=1"}
	args := []any{}

	if f.Location != "" {
		where = append(where, "j.location LIKE ?")
		args = append(args, "%"+f.Location+"%")
	}

	switch f.RemoteRegion {
	case "any":
		where = append(where, "j.remote_region != ''")
	case "us", "eu", "global":
		where = append(where, "j.remote_region = ?")
		args = append(args, f.RemoteRegion)
	}

	if f.SalaryMin != nil {
		where = append(where, "(j.salary_min >= ? OR j.salary_max >= ?)")
		args = append(args, *f.SalaryMin, *f.SalaryMin)
	}

	if f.DateFrom != "" {
		t, err := time.Parse("2006-01-02", f.DateFrom)
		if err == nil {
			where = append(where, "j.posted_at >= ?")
			args = append(args, t)
		}
	}

	whereClause := strings.Join(where, " AND ")

	var (
		total    int
		countSQL string
		countArgs []any
	)

	if useKeywords {
		// FTS JOIN so bm25() is available; MATCH arg goes first
		countArgs = append([]any{ftsQuery}, args...)
		countSQL = fmt.Sprintf(`
			SELECT COUNT(*) FROM jobs_fts
			JOIN jobs j ON j.id = jobs_fts.rowid
			WHERE jobs_fts MATCH ? AND %s`, whereClause)
	} else {
		countArgs = args
		countSQL = fmt.Sprintf(`SELECT COUNT(*) FROM jobs j WHERE %s`, whereClause)
	}

	err := s.db.QueryRow(countSQL, countArgs...).Scan(&total)
	if err != nil {
		return models.JobsResponse{}, err
	}

	offset := (f.Page - 1) * f.PageSize

	var (
		dataRows *sql.Rows
		cols     = `j.id, j.hn_item_id, j.thread_id, j.author, j.text, j.company,
		       j.location, j.remote_region, j.salary_min, j.salary_max, j.salary_curr,
		       j.posted_at, j.created_at, j.url`
	)

	if useKeywords {
		dataArgs := append([]any{ftsQuery}, args...)
		dataArgs = append(dataArgs, f.PageSize, offset)
		dataRows, err = s.db.Query(fmt.Sprintf(`
			SELECT %s
			FROM jobs_fts
			JOIN jobs j ON j.id = jobs_fts.rowid
			WHERE jobs_fts MATCH ? AND %s
			ORDER BY j.posted_at DESC
			LIMIT ? OFFSET ?
		`, cols, whereClause), dataArgs...)
	} else {
		dataArgs := append(args, f.PageSize, offset)
		dataRows, err = s.db.Query(fmt.Sprintf(`
			SELECT %s
			FROM jobs j
			WHERE %s
			ORDER BY j.posted_at DESC
			LIMIT ? OFFSET ?
		`, cols, whereClause), dataArgs...)
	}

	if err != nil {
		return models.JobsResponse{}, err
	}
	rows := dataRows
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var j models.Job
		var sMin, sMax sql.NullInt64
		err := rows.Scan(
			&j.ID, &j.HNItemID, &j.ThreadID, &j.Author, &j.Text, &j.Company,
			&j.Location, &j.RemoteRegion, &sMin, &sMax, &j.SalaryCurr,
			&j.PostedAt, &j.CreatedAt, &j.URL,
		)
		if err != nil {
			return models.JobsResponse{}, err
		}
		if sMin.Valid {
			v := int(sMin.Int64)
			j.SalaryMin = &v
		}
		if sMax.Valid {
			v := int(sMax.Int64)
			j.SalaryMax = &v
		}
		jobs = append(jobs, j)
	}

	totalPages := (total + f.PageSize - 1) / f.PageSize
	return models.JobsResponse{
		Jobs:       jobs,
		Total:      total,
		Page:       f.Page,
		PageSize:   f.PageSize,
		TotalPages: totalPages,
	}, nil
}

type techKeyword struct {
	label   string
	pattern string // custom regexp; empty = case-insensitive word boundary on label
}

// techKeywords defines the tracked technologies.
// Use a custom pattern when simple case-insensitive matching produces false positives.
var techKeywords = []techKeyword{
	{label: "React"}, {label: "Vue"}, {label: "Angular"}, {label: "Svelte"}, {label: "Next.js"},
	{label: "TypeScript"}, {label: "JavaScript"}, {label: "Python"},
	// "Go" is a common verb — match capitalized "Go" or "golang" (case-insensitive)
	{label: "Go", pattern: `\bGo\b|\bgolang\b`},
	{label: "Rust"}, {label: "Java"}, {label: "Kotlin"},
	{label: "Swift"}, {label: "Ruby"}, {label: "PHP"}, {label: "Scala"},
	{label: "Elixir"}, {label: "C++"}, {label: "C#"}, {label: "Haskell"},
	{label: "Node.js"}, {label: "Django"}, {label: "FastAPI"}, {label: "Rails"},
	{label: "Laravel"}, {label: "Spring"},
	{label: "PostgreSQL"}, {label: "MySQL"}, {label: "MongoDB"}, {label: "Redis"},
	{label: "Elasticsearch"}, {label: "SQLite"},
	{label: "AWS"}, {label: "GCP"}, {label: "Azure"}, {label: "Docker"},
	{label: "Kubernetes"}, {label: "Terraform"},
	{label: "GraphQL"}, {label: "gRPC"}, {label: "Kafka"}, {label: "Spark"}, {label: "Airflow"},
	{label: "PyTorch"}, {label: "TensorFlow"}, {label: "LLM"},
	// "AI" as standalone word; case-sensitive avoids "RAID", "email", etc.
	{label: "AI", pattern: `\bAI\b`},
	{label: "Machine Learning"}, {label: "Flutter"}, {label: "React Native"},
}

// kwRegexps maps each label to a precompiled regexp.
var kwRegexps = func() map[string]*regexp.Regexp {
	m := make(map[string]*regexp.Regexp, len(techKeywords))
	for _, kw := range techKeywords {
		pat := kw.pattern
		if pat == "" {
			pat = `(?i)\b` + regexp.QuoteMeta(kw.label) + `\b`
		}
		m[kw.label] = regexp.MustCompile(pat)
	}
	return m
}()

const trendsTTL = time.Hour

func (s *Store) GetTrends() (models.TrendsResponse, error) {
	s.trendsMu.Lock()
	defer s.trendsMu.Unlock()

	if s.trendsCache != nil && time.Since(s.trendsCachedAt) < trendsTTL {
		return *s.trendsCache, nil
	}

	result, err := s.computeTrends()
	if err != nil {
		return models.TrendsResponse{}, err
	}
	s.trendsCache = &result
	s.trendsCachedAt = time.Now()
	return result, nil
}

func (s *Store) computeTrends() (models.TrendsResponse, error) {
	// Load all job texts with their thread month in one query
	rows, err := s.db.Query(`
		SELECT j.text, t.month
		FROM jobs j
		JOIN threads t ON t.id = j.thread_id
		ORDER BY t.month
	`)
	if err != nil {
		return models.TrendsResponse{}, err
	}
	defer rows.Close()

	// counts[label][month] = count
	counts := make(map[string]map[string]int)
	for _, kw := range techKeywords {
		counts[kw.label] = make(map[string]int)
	}
	monthSeen := make(map[string]bool)

	for rows.Next() {
		var text, month string
		if err := rows.Scan(&text, &month); err != nil {
			continue
		}
		monthSeen[month] = true
		for _, kw := range techKeywords {
			if kwRegexps[kw.label].MatchString(text) {
				counts[kw.label][month]++
			}
		}
	}

	// Collect and sort months chronologically
	months := make([]string, 0, len(monthSeen))
	for m := range monthSeen {
		months = append(months, m)
	}
	sort.Slice(months, func(i, j int) bool {
		return parseMonth(months[i]).Before(parseMonth(months[j]))
	})

	// Build entries with totals
	entries := make([]models.TrendEntry, 0, len(techKeywords))
	for _, kw := range techKeywords {
		total := 0
		points := make([]models.TrendPoint, 0, len(months))
		for _, m := range months {
			c := counts[kw.label][m]
			total += c
			points = append(points, models.TrendPoint{Month: m, Count: c})
		}
		entries = append(entries, models.TrendEntry{Keyword: kw.label, Total: total, Points: points})
	}

	// Sort by total descending, take top 20
	sort.Slice(entries, func(i, j int) bool { return entries[i].Total > entries[j].Total })
	if len(entries) > 20 {
		entries = entries[:20]
	}

	return models.TrendsResponse{Trends: entries, Months: months}, nil
}

// parseMonth parses "January 2025" into a time.Time for chronological sorting.
func parseMonth(s string) time.Time {
	t, _ := time.Parse("January 2006", s)
	return t
}

func (s *Store) GetLatestThreadItemID() (int, error) {
	var id int
	err := s.db.QueryRow(`SELECT hn_item_id FROM threads ORDER BY created_at DESC LIMIT 1`).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return id, err
}
