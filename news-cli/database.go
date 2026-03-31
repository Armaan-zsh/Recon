package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// IntelligenceDB manages the persistent SQLite store for the Nexus.
type IntelligenceDB struct {
	db *sql.DB
}

// InitDB initializes the SQLite database with WAL mode and necessary schema.
func InitDB() (*IntelligenceDB, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config dir: %w", err)
	}

	appConfigDir := filepath.Join(configDir, appName)
	if _, err := os.Stat(appConfigDir); os.IsNotExist(err) {
		if err := os.MkdirAll(appConfigDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create app config dir: %w", err)
		}
	}

	dbPath := filepath.Join(appConfigDir, "nexus.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	// Enable WAL mode for concurrent Read/Write
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Initialize Schema
	schema := `
	CREATE TABLE IF NOT EXISTS articles (
		hash TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		link TEXT NOT NULL,
		published_at DATETIME,
		source_name TEXT,
		score INTEGER,
		summary TEXT,
		cluster_id TEXT,
		seen BOOLEAN DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS entities (
		name TEXT PRIMARY KEY,
		type TEXT -- MALWARE, APT, CVE, TARGET, INFRA
	);

	CREATE TABLE IF NOT EXISTS article_entities (
		article_hash TEXT,
		entity_name TEXT,
		PRIMARY KEY(article_hash, entity_name),
		FOREIGN KEY(article_hash) REFERENCES articles(hash),
		FOREIGN KEY(entity_name) REFERENCES entities(name)
	);

	CREATE TABLE IF NOT EXISTS knowledge_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		article_hash TEXT UNIQUE,
		seen_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS system_state (
		key TEXT PRIMARY KEY,
		value TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_articles_published ON articles(published_at);
	CREATE INDEX IF NOT EXISTS idx_entities_name ON entities(name);
	`
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &IntelligenceDB{db: db}, nil
}

// Close closes the database connection.
func (i *IntelligenceDB) Close() error {
	return i.db.Close()
}

// SaveArticle persists an article and its relationships.
func (i *IntelligenceDB) SaveArticle(art Article, entities []string) error {
	tx, err := i.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Save Article
	_, err = tx.Exec(`
		INSERT INTO articles (hash, title, link, published_at, source_name, score, summary)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(hash) DO UPDATE SET
			score = excluded.score,
			summary = excluded.summary;
	`, art.Hash(), art.Title, art.Link, art.Published, art.SourceName, art.Score, art.Description)
	if err != nil {
		return err
	}

	// 2. Save Entities and Relationships
	for _, ent := range entities {
		_, err = tx.Exec("INSERT OR IGNORE INTO entities (name, type) VALUES (?, ?)", ent, "UNKNOWN")
		if err != nil {
			return err
		}
		_, err = tx.Exec("INSERT OR IGNORE INTO article_entities (article_hash, entity_name) VALUES (?, ?)", art.Hash(), ent)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetArticleEntities returns all entities associated with an article.
func (i *IntelligenceDB) GetArticleEntities(hash string) ([]string, error) {
	rows, err := i.db.Query("SELECT entity_name FROM article_entities WHERE article_hash = ?", hash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		entities = append(entities, name)
	}
	return entities, nil
}

// GetEntityTimeline returns all articles associated with an entity, sorted by time.
func (i *IntelligenceDB) GetEntityTimeline(entityName string) ([]Article, error) {
	rows, err := i.db.Query(`
		SELECT a.title, a.link, a.published_at, a.source_name, a.score, a.summary
		FROM articles a
		JOIN article_entities ae ON a.hash = ae.article_hash
		WHERE ae.entity_name = ?
		ORDER BY a.published_at ASC
	`, entityName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var a Article
		var publishedAt time.Time
		if err := rows.Scan(&a.Title, &a.Link, &publishedAt, &a.SourceName, &a.Score, &a.Description); err != nil {
			return nil, err
		}
		a.Published = publishedAt
		articles = append(articles, a)
	}
	return articles, nil
}

// GetRecentArticles returns recently fetched articles ordered by score and date.
func (i *IntelligenceDB) GetRecentArticles(limit int) ([]Article, error) {
	rows, err := i.db.Query(`
		SELECT title, link, published_at, source_name, score, summary
		FROM articles
		ORDER BY score DESC, published_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var a Article
		var publishedAt time.Time
		if err := rows.Scan(&a.Title, &a.Link, &publishedAt, &a.SourceName, &a.Score, &a.Description); err != nil {
			return nil, err
		}
		a.Published = publishedAt
		articles = append(articles, a)
	}
	return articles, nil
}

// GetLastSyncTime retrieves the last successful fetch timestamp to enable debouncing.
func (i *IntelligenceDB) GetLastSyncTime() time.Time {
	var val string
	err := i.db.QueryRow("SELECT value FROM system_state WHERE key = 'last_sync'").Scan(&val)
	if err != nil {
		return time.Time{} // Return zero-time if never synced
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return time.Time{}
	}
	return t
}

// SetLastSyncTime updates the sync debounce lock.
func (i *IntelligenceDB) SetLastSyncTime(t time.Time) error {
	_, err := i.db.Exec(`
		INSERT INTO system_state (key, value) VALUES ('last_sync', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value;
	`, t.Format(time.RFC3339))
	return err
}
// trying to commit this checking if this works or not
