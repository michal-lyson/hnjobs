package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/lyson/hn-jobs/internal/db"
	"github.com/lyson/hn-jobs/internal/models"
)

type Handler struct {
	store *db.Store
}

func NewRouter(store *db.Store) http.Handler {
	h := &Handler{store: store}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type"},
	}))

	r.Get("/api/jobs", h.listJobs)
	r.Get("/api/trends", h.trends)
	r.Get("/api/health", h.health)

	return r
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	f := models.JobFilter{
		Keywords:     q.Get("keywords"),
		Location:     q.Get("location"),
		DateFrom:     q.Get("date_from"),
		RemoteRegion: q.Get("remote_region"), // "any", "us", "eu", "global", ""
	}

	if v := q.Get("salary_min"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.SalaryMin = &n
		}
	}

	if v := q.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Page = n
		}
	}

	if v := q.Get("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n <= 100 {
			f.PageSize = n
		}
	}

	resp, err := h.store.ListJobs(f)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) trends(w http.ResponseWriter, r *http.Request) {
	resp, err := h.store.GetTrends()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

var _ = writeJSON // suppress unused warning
