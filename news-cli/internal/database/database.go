package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
	"news-cli/internal/models"
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
		type TEXT -- MALWARE, APT, CVE, TARGET, INFRA, NATION_STATE
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
	CREATE INDEX IF NOT EXISTS idx_articles_score ON articles(score DESC);
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
	`, art.Hash, art.Title, art.Link, art.Published, art.SourceName, art.Score, art.Description)
	if err != nil {
		return err
	}

	for _, ent := range entities {
		_, err = tx.Exec("INSERT OR IGNORE INTO entities (name, type) VALUES (?, ?)", ent, "UNKNOWN")
		if err != nil {
			return err
		}
		_, err = tx.Exec("INSERT OR IGNORE INTO article_entities (article_hash, entity_name) VALUES (?, ?)", art.Hash, ent)
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

// GetRecentArticles returns recently fetched articles ordered by score and date.
func (i *IntelligenceDB) GetRecentArticles(limit int) ([]models.Article, error) {
	rows, err := i.db.Query(`
		SELECT title, link, published_at, source_name, score, summary
		FROM articles
		WHERE substr(published_at, 1, 19) <= datetime('now', '+36 hours')
		ORDER BY (score * 1.0 / power(((strftime('%s','now') - strftime('%s', substr(published_at, 1, 19)))/3600.0) + 2, 1.8)) DESC
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

	return articles, nil
}

// SearchArticles searches for articles matching a query with optional filters.
func (i *IntelligenceDB) SearchArticles(query string, afterDate time.Time, minScore int) ([]models.Article, error) {
	rows, err := i.db.Query(`
		SELECT title, link, published_at, source_name, score, summary
		FROM articles
		WHERE (title LIKE ? OR summary LIKE ?)
		AND published_at >= ?
		AND score >= ?
		ORDER BY score DESC, published_at DESC
	`, "%"+query+"%", "%"+query+"%", afterDate.Format("2006-01-02"), minScore)
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

// GetArticlesByCategory returns articles filtered by category keywords.
func (i *IntelligenceDB) GetArticlesByCategory(keywords []string, source string) ([]models.Article, error) {
	if len(keywords) == 0 {
		return nil, fmt.Errorf("no keywords provided")
	}

	// Build dynamic query with multiple keyword conditions
	query := `
		SELECT title, link, published_at, source_name, score, summary
		FROM articles
		WHERE (`
	
	conditions := make([]string, len(keywords))
	args := make([]interface{}, len(keywords))
	for i, kw := range keywords {
		conditions[i] = "(title LIKE ? OR summary LIKE ?)"
		args[i*2] = "%" + kw + "%"
		args[i*2+1] = "%" + kw + "%"
	}
	
	query += " OR "
	query += conditions[0]
	for _, cond := range conditions[1:] {
		query += " OR " + cond
	}
	query += ")"

	if source != "" {
		query += " AND source_name = ?"
		args = append(args, source)
	}

	query += " ORDER BY score DESC, published_at DESC"

	rows, err := i.db.Query(query, args...)
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

// SetLastSyncTime updates the sync debounce lock.
func (i *IntelligenceDB) SetLastSyncTime(t time.Time) error {
	_, err := i.db.Exec(`
		INSERT INTO system_state (key, value) VALUES ('last_sync', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value;
	`, t.Format(time.RFC3339))
	return err
}

// GetAllEntities returns all known entities with their types.
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

// GetTrendingEntities returns the most frequently mentioned entities in recent articles.
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
