package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	log.Printf("database opened at %s", path)
	return db, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS threads (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			hn_item_id INTEGER UNIQUE NOT NULL,
			title      TEXT NOT NULL,
			month      TEXT NOT NULL,
			scraped_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS jobs (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			hn_item_id    INTEGER UNIQUE NOT NULL,
			thread_id     INTEGER NOT NULL REFERENCES threads(id),
			author        TEXT NOT NULL,
			text          TEXT NOT NULL,
			company       TEXT NOT NULL DEFAULT '',
			location      TEXT NOT NULL DEFAULT '',
			remote_region TEXT NOT NULL DEFAULT '',
			salary_min    INTEGER,
			salary_max    INTEGER,
			salary_curr   TEXT NOT NULL DEFAULT '',
			posted_at     DATETIME NOT NULL,
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			url           TEXT NOT NULL DEFAULT ''
		);

		CREATE INDEX IF NOT EXISTS idx_jobs_posted_at ON jobs(posted_at DESC);
		CREATE INDEX IF NOT EXISTS idx_jobs_thread_id ON jobs(thread_id);
		CREATE INDEX IF NOT EXISTS idx_jobs_salary_min ON jobs(salary_min);

		CREATE VIRTUAL TABLE IF NOT EXISTS jobs_fts USING fts5(
			text,
			company,
			location,
			content=jobs,
			content_rowid=id
		);

		CREATE TRIGGER IF NOT EXISTS jobs_ai AFTER INSERT ON jobs BEGIN
			INSERT INTO jobs_fts(rowid, text, company, location)
			VALUES (new.id, new.text, new.company, new.location);
		END;

		CREATE TRIGGER IF NOT EXISTS jobs_ad AFTER DELETE ON jobs BEGIN
			INSERT INTO jobs_fts(jobs_fts, rowid, text, company, location)
			VALUES ('delete', old.id, old.text, old.company, old.location);
		END;

		CREATE TRIGGER IF NOT EXISTS jobs_au AFTER UPDATE ON jobs BEGIN
			INSERT INTO jobs_fts(jobs_fts, rowid, text, company, location)
			VALUES ('delete', old.id, old.text, old.company, old.location);
			INSERT INTO jobs_fts(rowid, text, company, location)
			VALUES (new.id, new.text, new.company, new.location);
		END;
	`)
	if err != nil {
		return err
	}

	// v2: replace remote boolean with remote_region text (for existing databases)
	addColumnIfMissing(db, "jobs", "remote_region", "TEXT NOT NULL DEFAULT ''")
	// v3: track when a thread was fully scraped
	addColumnIfMissing(db, "threads", "scraped_at", "DATETIME")

	// Index on remote_region — created after the column is guaranteed to exist
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_jobs_remote_region ON jobs(remote_region)`); err != nil {
		return err
	}

	return nil
}

// addColumnIfMissing runs ALTER TABLE … ADD COLUMN and ignores errors (e.g. column already exists).
func addColumnIfMissing(db *sql.DB, table, column, def string) {
	db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, def)) //nolint
}
