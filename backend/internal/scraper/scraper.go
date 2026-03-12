package scraper

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/lyson/hn-jobs/internal/db"
	"github.com/lyson/hn-jobs/internal/models"
)

const (
	hnAPIBase    = "https://hacker-news.firebaseio.com/v0"
	hnSearchBase = "https://hn.algolia.com/api/v1"
)

type Scraper struct {
	store  *db.Store
	client *http.Client
}

func New(store *db.Store) *Scraper {
	return &Scraper{
		store:  store,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type hnItem struct {
	ID    int    `json:"id"`
	By    string `json:"by"`
	Text  string `json:"text"`
	Time  int64  `json:"time"`
	Kids  []int  `json:"kids"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Type  string `json:"type"`
}

type algoliaResponse struct {
	Hits []struct {
		ObjectID  string `json:"objectID"`
		Title     string `json:"title"`
		CreatedAt string `json:"created_at"`
	} `json:"hits"`
}

func (s *Scraper) Run() {
	log.Println("scraper: starting run")

	threadIDs, err := s.findHiringThreads()
	if err != nil {
		log.Printf("scraper: find threads error: %v", err)
		return
	}

	log.Printf("scraper: found %d hiring threads", len(threadIDs))

	for _, threadID := range threadIDs {
		done, err := s.scrapeThread(threadID)
		if err != nil {
			log.Printf("scraper: thread %d error: %v", threadID, err)
		}
		if done {
			log.Println("scraper: reached old scraped thread — stopping early")
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	log.Println("scraper: run complete")
}

func (s *Scraper) findHiringThreads() ([]int, error) {
	// Use Algolia HN search to find "Who is Hiring?" threads
	params := url.Values{}
	params.Set("query", `"Ask HN: Who is hiring"`)
	params.Set("tags", "story,author_whoishiring")
	params.Set("hitsPerPage", "36")
	reqURL := fmt.Sprintf("%s/search_by_date?%s", hnSearchBase, params.Encode())
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result algoliaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var ids []int
	for _, hit := range result.Hits {
		id, err := strconv.Atoi(hit.ObjectID)
		if err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// scrapeThreshold is how old a thread must be before we consider it frozen
// (no new comments expected) and skip re-scraping if already done.
const scrapeThreshold = 45 * 24 * time.Hour

// scrapeThread processes one thread. Returns (true, nil) when the thread is
// old enough to be considered frozen — the caller should stop iterating.
func (s *Scraper) scrapeThread(threadID int) (done bool, err error) {
	item, err := s.fetchItem(threadID)
	if err != nil {
		return false, fmt.Errorf("fetch thread: %w", err)
	}

	month := extractMonth(item.Title)
	threadDBID, alreadyScraped, err := s.store.UpsertThread(item.ID, item.Title, month)
	if err != nil {
		return false, fmt.Errorf("upsert thread: %w", err)
	}

	threadAge := time.Since(time.Unix(item.Time, 0))
	if alreadyScraped && threadAge > scrapeThreshold {
		log.Printf("scraper: skipping thread %d (%s) — already scraped and >45 days old", threadID, item.Title)
		return true, nil
	}

	log.Printf("scraper: processing thread %d (%s) with %d comments", threadID, item.Title, len(item.Kids))

	for _, kidID := range item.Kids {
		kid, err := s.fetchItem(kidID)
		if err != nil {
			log.Printf("scraper: fetch comment %d: %v", kidID, err)
			continue
		}

		if kid.Text == "" {
			continue
		}

		job := parseJob(kid, int(threadDBID))
		if err := s.store.UpsertJob(job); err != nil {
			log.Printf("scraper: upsert job %d: %v", kidID, err)
		}

		time.Sleep(100 * time.Millisecond)
	}

	if err := s.store.MarkThreadScraped(threadDBID); err != nil {
		log.Printf("scraper: mark thread %d scraped: %v", threadID, err)
	}

	return false, nil
}

func (s *Scraper) fetchItem(id int) (*hnItem, error) {
	url := fmt.Sprintf("%s/item/%d.json", hnAPIBase, id)
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var item hnItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

func parseJob(item *hnItem, threadID int) *models.Job {
	text := htmlToText(item.Text)

	job := &models.Job{
		HNItemID: item.ID,
		ThreadID: threadID,
		Author:   item.By,
		Text:     text,
		PostedAt: time.Unix(item.Time, 0),
		URL:      fmt.Sprintf("https://news.ycombinator.com/item?id=%d", item.ID),
	}

	job.Company = extractCompany(text)
	job.Location, job.RemoteRegion = extractLocation(text)
	job.SalaryMin, job.SalaryMax, job.SalaryCurr = extractSalary(text)

	return job
}

func htmlToText(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<body>" + html + "</body>"))
	if err != nil {
		return html
	}
	doc.Find("p").Each(func(_ int, s *goquery.Selection) {
		s.ReplaceWithHtml("\n" + s.Text() + "\n")
	})
	text := doc.Find("body").Text()
	// Clean up excessive whitespace
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			cleaned = append(cleaned, l)
		}
	}
	return strings.Join(cleaned, "\n")
}

var (
	// Match "Company Name | ..." or first line
	companyRe = regexp.MustCompile(`(?i)^([A-Z][^|\n]{2,60})\s*[|\-]`)

	locationRe = regexp.MustCompile(`(?i)\b(remote|onsite|on-site|hybrid|(?:san francisco|sf|new york|nyc|london|berlin|toronto|austin|seattle|boston|chicago|los angeles|la|amsterdam|paris|singapore|tokyo|sydney|bangalore|remote)[^,\n|]*(?:,\s*[A-Z]{2})?)\b`)

	remoteRe = regexp.MustCompile(`(?i)\bremote\b`)

	// Remote region classifiers — checked in order (US before EU before global)
	remoteUSRe = regexp.MustCompile(`(?i)\bremote\b.{0,40}\b(us|usa|u\.s\.?|united states|north america|us only|us-only)\b|\b(us|usa|united states|north america|us only)\b.{0,40}\bremote\b`)
	remoteEURe = regexp.MustCompile(`(?i)\bremote\b.{0,40}\b(eu|europe|european|emea|uk|united kingdom|germany|france|netherlands)\b|\b(eu|europe|european|emea|uk|united kingdom)\b.{0,40}\bremote\b`)

	// Captures: [1]=symbol/code, [2]=min amount, [3]=max amount (optional)
	// Supports: $, €, £, ¥, CHF, CAD, AUD, SEK, DKK, NOK, PLN, CZK, SGD, INR
	salaryRe = regexp.MustCompile(`(?i)(€|£|\$|¥|CHF|CAD|AUD|SEK|DKK|NOK|PLN|CZK|SGD|INR|RS\.?)\s*(\d{2,4})[Kk]?\s*(?:[-–]\s*(?:€|£|\$|¥|CHF|CAD|AUD|SEK|DKK|NOK|PLN|CZK|SGD|INR|RS\.?)?\s*(\d{2,4})[Kk]?)?\s*(?:/\s*(?:yr|year|annum))?`)

	currencyMap = map[string]string{
		"$": "USD", "€": "EUR", "£": "GBP", "¥": "JPY",
		"chf": "CHF", "cad": "CAD", "aud": "AUD", "sek": "SEK",
		"dkk": "DKK", "nok": "NOK", "pln": "PLN", "czk": "CZK",
		"sgd": "SGD", "inr": "INR", "rs": "INR", "rs.": "INR",
	}
)

func extractCompany(text string) string {
	// Try pipe or dash separator on first line
	firstLine := strings.SplitN(text, "\n", 2)[0]
	if m := companyRe.FindStringSubmatch(firstLine); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// Fallback: first line up to 60 chars
	if len(firstLine) > 60 {
		firstLine = firstLine[:60]
	}
	return strings.TrimSpace(firstLine)
}

// extractLocation returns the location string and remote region ("us", "eu", "global", "").
func extractLocation(text string) (string, string) {
	locs := locationRe.FindAllString(text, 3)

	var unique []string
	seen := map[string]bool{}
	for _, l := range locs {
		l = strings.TrimSpace(l)
		low := strings.ToLower(l)
		if !seen[low] {
			seen[low] = true
			unique = append(unique, l)
		}
	}

	var remoteRegion string
	if remoteRe.MatchString(text) {
		switch {
		case remoteUSRe.MatchString(text):
			remoteRegion = "us"
		case remoteEURe.MatchString(text):
			remoteRegion = "eu"
		default:
			remoteRegion = "global"
		}
	}

	return strings.Join(unique, ", "), remoteRegion
}

func extractSalary(text string) (*int, *int, string) {
	m := salaryRe.FindStringSubmatch(text)
	if m == nil {
		return nil, nil, ""
	}
	// m[1]=symbol, m[2]=min, m[3]=max (optional)
	symbol := strings.ToLower(strings.TrimSpace(m[1]))
	currency := currencyMap[symbol]
	if currency == "" {
		currency = strings.ToUpper(symbol)
	}

	minVal, err := strconv.Atoi(m[2])
	if err != nil {
		return nil, nil, ""
	}
	if minVal < 1000 {
		minVal *= 1000
	}
	min := minVal

	var maxResult *int
	if m[3] != "" {
		maxVal, err := strconv.Atoi(m[3])
		if err == nil {
			if maxVal < 1000 {
				maxVal *= 1000
			}
			maxResult = &maxVal
		}
	}

	return &min, maxResult, currency
}

func extractMonth(title string) string {
	// e.g. "Ask HN: Who is hiring? (March 2024)"
	re := regexp.MustCompile(`\(([A-Za-z]+ \d{4})\)`)
	if m := re.FindStringSubmatch(title); len(m) > 1 {
		return m[1]
	}
	return ""
}
