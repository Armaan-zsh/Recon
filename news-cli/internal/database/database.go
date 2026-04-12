package database

import (
	"database/sql"
	"fmt"
	"math"
	"news-cli/internal/clusterer"
	"news-cli/internal/models"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

type IntelligenceDB struct {
	db *sql.DB
}

func InitDB() (*IntelligenceDB, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config dir: %w", err)
	}

	appConfigDir := filepath.Join(configDir, "recon")
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

	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

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
		type TEXT
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

	-- FTS5 Search Table
	CREATE VIRTUAL TABLE IF NOT EXISTS article_search USING fts5(
		hash UNINDEXED,
		title,
		summary,
		source_name UNINDEXED,
		content='articles',
		content_rowid='hash'
	);

	-- Triggers to keep FTS5 in sync
	CREATE TRIGGER IF NOT EXISTS trg_articles_ai AFTER INSERT ON articles BEGIN
		INSERT INTO article_search(rowid, title, summary) VALUES (new.hash, new.title, new.summary);
	END;
	CREATE TRIGGER IF NOT EXISTS trg_articles_ad AFTER DELETE ON articles BEGIN
		INSERT INTO article_search(article_search, rowid, title, summary) VALUES('delete', old.hash, old.title, old.summary);
	END;
	CREATE TRIGGER IF NOT EXISTS trg_articles_au AFTER UPDATE ON articles BEGIN
		INSERT INTO article_search(article_search, rowid, title, summary) VALUES('delete', old.hash, old.title, old.summary);
		INSERT INTO article_search(rowid, title, summary) VALUES(new.hash, new.title, new.summary);
	END;

	CREATE INDEX IF NOT EXISTS idx_articles_published ON articles(published_at);
	CREATE INDEX IF NOT EXISTS idx_entities_name ON entities(name);
	CREATE INDEX IF NOT EXISTS idx_articles_score ON articles(score DESC);
	`
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &IntelligenceDB{db: db}, nil
}

func (i *IntelligenceDB) Close() error {
	return i.db.Close()
}

func (i *IntelligenceDB) SaveArticle(art models.Article, entities []string) error {
	tx, err := i.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO articles (hash, title, link, published_at, source_name, score, summary)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(hash) DO UPDATE SET
			score = excluded.score,
			summary = excluded.summary;
	`, art.Hash(), art.Title, art.Link, art.Published.UTC().Format("2006-01-02 15:04:05"), art.SourceName, art.Score, art.Description)
	if err != nil {
		return err
	}

	for _, ent := range entities {
		_, _ = tx.Exec("INSERT OR IGNORE INTO entities (name, type) VALUES (?, ?)", ent, "UNKNOWN")
		_, _ = tx.Exec("INSERT OR IGNORE INTO article_entities (article_hash, entity_name) VALUES (?, ?)", art.Hash(), ent)
	}

	return tx.Commit()
}

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

func (i *IntelligenceDB) GetEntityTimeline(entityName string) ([]models.Article, error) {
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

	var articles []models.Article
	for rows.Next() {
		var a models.Article
		var publishedAt time.Time
		if err := rows.Scan(&a.Title, &a.Link, &publishedAt, &a.SourceName, &a.Score, &a.Description); err != nil {
			return nil, err
		}
		a.Published = publishedAt
		articles = append(articles, a)
	}
	return articles, nil
}

func (i *IntelligenceDB) GetRecentArticles(limit int) ([]models.Article, error) {
	rows, err := i.db.Query(`
		SELECT title, link, published_at, source_name, score, summary
		FROM articles
		WHERE published_at >= datetime('now', '-48 hours')
		ORDER BY (score * 1.0 / power(((strftime('%s','now') - strftime('%s', published_at))/3600.0) + 2, 1.8)) DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		var a models.Article
		var publishedAt time.Time
		if err := rows.Scan(&a.Title, &a.Link, &publishedAt, &a.SourceName, &a.Score, &a.Description); err != nil {
			return nil, err
		}
		a.Published = publishedAt
		articles = append(articles, a)
	}

	c := clusterer.NewClusterer(3)
	groups := c.ClusterArticles(articles)
	var unique []models.Article
	for _, g := range groups {
		unique = append(unique, g.PrimaryArticle)
	}

	sort.Slice(unique, func(i, j int) bool {
		ageI := time.Since(unique[i].Published).Hours()
		ageJ := time.Since(unique[j].Published).Hours()
		scoreI := float64(unique[i].Score) / math.Pow(ageI+2, 1.8)
		scoreJ := float64(unique[j].Score) / math.Pow(ageJ+2, 1.8)
		return scoreI > scoreJ
	})

	return unique, nil
}

func (i *IntelligenceDB) SearchArticles(query string, afterDate time.Time, minScore int) ([]models.Article, error) {
	rows, err := i.db.Query(`
		SELECT a.title, a.link, a.published_at, a.source_name, a.score, a.summary
		FROM articles a
		JOIN article_search s ON a.hash = s.rowid
		WHERE article_search MATCH ?
		AND a.published_at >= ?
		AND a.score >= ?
		ORDER BY rank, a.published_at DESC
	`, query, afterDate.Format("2006-01-02"), minScore)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		var a models.Article
		var publishedAt time.Time
		if err := rows.Scan(&a.Title, &a.Link, &publishedAt, &a.SourceName, &a.Score, &a.Description); err != nil {
			return nil, err
		}
		a.Published = publishedAt
		articles = append(articles, a)
	}
	return articles, nil
}

func (i *IntelligenceDB) GetLastSyncTime() time.Time {
	var val string
	err := i.db.QueryRow("SELECT value FROM system_state WHERE key = 'last_sync'").Scan(&val)
	if err != nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (i *IntelligenceDB) SetLastSyncTime(t time.Time) error {
	_, err := i.db.Exec(`
		INSERT INTO system_state (key, value) VALUES ('last_sync', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value;
	`, t.Format(time.RFC3339))
	return err
}

func (i *IntelligenceDB) GetAllEntities() ([]models.Entity, error) {
	rows, err := i.db.Query("SELECT name, type FROM entities ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []models.Entity
	for rows.Next() {
		var e models.Entity
		if err := rows.Scan(&e.Name, &e.Type); err != nil {
			return nil, err
		}
		entities = append(entities, e)
	}
	return entities, nil
}

func (i *IntelligenceDB) GetTrendingEntities(hours int, limit int) ([]models.Entity, error) {
	rows, err := i.db.Query(`
		SELECT e.name, e.type, COUNT(*) as mention_count
		FROM entities e
		JOIN article_entities ae ON e.name = ae.entity_name
		JOIN articles a ON ae.article_hash = a.hash
		WHERE a.published_at >= datetime('now', '-' || ? || ' hours')
		GROUP BY e.name, e.type
		ORDER BY mention_count DESC
		LIMIT ?
	`, hours, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []models.Entity
	for rows.Next() {
		var e models.Entity
		var count int
		if err := rows.Scan(&e.Name, &e.Type, &count); err != nil {
			return nil, err
		}
		entities = append(entities, e)
	}
	return entities, nil
}
