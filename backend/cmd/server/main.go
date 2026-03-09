package main

import (
	"log"
	"net/http"
	"os"

	"github.com/robfig/cron/v3"

	"github.com/lyson/hn-jobs/internal/api"
	"github.com/lyson/hn-jobs/internal/db"
	"github.com/lyson/hn-jobs/internal/scraper"
)

func main() {
	dbPath := getenv("DB_PATH", "./data/jobs.db")
	addr := getenv("ADDR", ":8080")

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	store := db.NewStore(database)
	sc := scraper.New(store)

	// Warm trends cache from existing DB data before accepting requests
	store.WarmCache()

	// Run scraper immediately on startup; invalidate cache when done so fresh
	// data is reflected on the next trends request
	go func() {
		sc.Run()
		store.InvalidateTrendsCache()
		store.WarmCache()
	}()

	// Schedule scraper to run daily at 9am
	c := cron.New()
	_, err = c.AddFunc("0 9 * * *", func() {
		sc.Run()
		store.InvalidateTrendsCache()
		store.WarmCache()
	})
	if err != nil {
		log.Fatalf("cron: %v", err)
	}
	c.Start()
	defer c.Stop()

	router := api.NewRouter(store)

	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
