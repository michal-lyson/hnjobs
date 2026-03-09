package models

import "time"

type Job struct {
	ID           int       `json:"id"`
	HNItemID     int       `json:"hn_item_id"`
	ThreadID     int       `json:"thread_id"`
	Author       string    `json:"author"`
	Text         string    `json:"text"`
	Company      string    `json:"company"`
	Location     string    `json:"location"`
	RemoteRegion string    `json:"remote_region"` // "us", "eu", "global", ""
	SalaryMin    *int      `json:"salary_min,omitempty"`
	SalaryMax    *int      `json:"salary_max,omitempty"`
	SalaryCurr   string    `json:"salary_currency"`
	Keywords     []string  `json:"keywords"`
	PostedAt     time.Time `json:"posted_at"`
	CreatedAt    time.Time `json:"created_at"`
	URL          string    `json:"url"`
}

type Thread struct {
	ID        int       `json:"id"`
	HNItemID  int       `json:"hn_item_id"`
	Title     string    `json:"title"`
	Month     string    `json:"month"`
	CreatedAt time.Time `json:"created_at"`
}

type JobFilter struct {
	Location     string `json:"location"`
	RemoteRegion string `json:"remote_region"` // "any", "us", "eu", "global", ""
	SalaryMin    *int   `json:"salary_min"`
	Keywords     string `json:"keywords"`
	DateFrom     string `json:"date_from"`
	Page         int    `json:"page"`
	PageSize     int    `json:"page_size"`
}

type JobsResponse struct {
	Jobs       []Job `json:"jobs"`
	Total      int   `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

type TrendPoint struct {
	Month string `json:"month"`
	Count int    `json:"count"`
}

type TrendEntry struct {
	Keyword string       `json:"keyword"`
	Total   int          `json:"total"`
	Points  []TrendPoint `json:"points"`
}

type TrendsResponse struct {
	Trends []TrendEntry `json:"trends"`
	Months []string     `json:"months"`
}
