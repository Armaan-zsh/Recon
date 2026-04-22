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

type ArchiveDay struct {
	Date  string
	Count int
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
	_, _ = db.Exec("PRAGMA busy_timeout=5000;")

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

	CREATE TABLE IF NOT EXISTS feed_cache (
		url TEXT PRIMARY KEY,
		etag TEXT,
		last_modified TEXT,
		last_fetched_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_articles_published ON articles(published_at);
	CREATE INDEX IF NOT EXISTS idx_entities_name ON entities(name);
	CREATE INDEX IF NOT EXISTS idx_articles_score ON articles(score DESC);
	`
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	var ftsVersion string
	_ = db.QueryRow("SELECT value FROM system_state WHERE key = 'fts_version'").Scan(&ftsVersion)

	if ftsVersion != "2" {
		ftsMigrate := `
		DROP TRIGGER IF EXISTS trg_articles_ai;
		DROP TRIGGER IF EXISTS trg_articles_ad;
		DROP TRIGGER IF EXISTS trg_articles_au;
		DROP TABLE IF EXISTS article_search;

		CREATE VIRTUAL TABLE article_search USING fts5(
			hash UNINDEXED,
			title,
			summary,
			source_name UNINDEXED
		);

		CREATE TRIGGER trg_articles_ai AFTER INSERT ON articles BEGIN
			INSERT INTO article_search(hash, title, summary, source_name) VALUES (new.hash, new.title, new.summary, new.source_name);
		END;
		CREATE TRIGGER trg_articles_ad AFTER DELETE ON articles BEGIN
			INSERT INTO article_search(article_search, hash, title, summary, source_name) VALUES('delete', old.hash, old.title, old.summary, old.source_name);
		END;
		CREATE TRIGGER trg_articles_au AFTER UPDATE ON articles BEGIN
			INSERT INTO article_search(article_search, hash, title, summary, source_name) VALUES('delete', old.hash, old.title, old.summary, old.source_name);
			INSERT INTO article_search(hash, title, summary, source_name) VALUES(new.hash, new.title, new.summary, new.source_name);
		END;

		INSERT INTO article_search(hash, title, summary, source_name)
		SELECT hash, title, summary, source_name FROM articles;
		`
		if _, err := db.Exec(ftsMigrate); err != nil {
			return nil, fmt.Errorf("failed to migrate FTS schema: %w", err)
		}
		_, _ = db.Exec("INSERT INTO system_state (key, value) VALUES ('fts_version', '2') ON CONFLICT(key) DO UPDATE SET value='2'")
	} else {
		ftsEnsure := `
		CREATE VIRTUAL TABLE IF NOT EXISTS article_search USING fts5(
			hash UNINDEXED,
			title,
			summary,
			source_name UNINDEXED
		);

		CREATE TRIGGER IF NOT EXISTS trg_articles_ai AFTER INSERT ON articles BEGIN
			INSERT INTO article_search(hash, title, summary, source_name) VALUES (new.hash, new.title, new.summary, new.source_name);
		END;
		CREATE TRIGGER IF NOT EXISTS trg_articles_ad AFTER DELETE ON articles BEGIN
			INSERT INTO article_search(article_search, hash, title, summary, source_name) VALUES('delete', old.hash, old.title, old.summary, old.source_name);
		END;
		CREATE TRIGGER IF NOT EXISTS trg_articles_au AFTER UPDATE ON articles BEGIN
			INSERT INTO article_search(article_search, hash, title, summary, source_name) VALUES('delete', old.hash, old.title, old.summary, old.source_name);
			INSERT INTO article_search(hash, title, summary, source_name) VALUES(new.hash, new.title, new.summary, new.source_name);
		END;
		`
		if _, err := db.Exec(ftsEnsure); err != nil {
			return nil, fmt.Errorf("failed to ensure FTS schema: %w", err)
		}
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

func (i *IntelligenceDB) GetFeedCache(url string) (string, string, error) {
	var etag, lastModified string
	err := i.db.QueryRow(`SELECT etag, last_modified FROM feed_cache WHERE url = ?`, url).Scan(&etag, &lastModified)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return etag, lastModified, err
}

func (i *IntelligenceDB) SetFeedCache(url string, etag string, lastModified string) error {
	_, err := i.db.Exec(`
		INSERT INTO feed_cache (url, etag, last_modified, last_fetched_at)
		VALUES (?, ?, ?, datetime('now'))
		ON CONFLICT(url) DO UPDATE SET
			etag = excluded.etag,
			last_modified = excluded.last_modified,
			last_fetched_at = excluded.last_fetched_at
	`, url, etag, lastModified)
	return err
}

func (i *IntelligenceDB) SearchArticles(query string, afterDate time.Time, minScore int) ([]models.Article, error) {
	rows, err := i.db.Query(`
		SELECT a.title, a.link, a.published_at, a.source_name, a.score, a.summary
		FROM articles a
		JOIN article_search s ON a.hash = s.hash
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
		if e.Type == "" || e.Type == "UNKNOWN" {
			if len(e.Name) >= 4 && e.Name[:4] == "CVE-" {
				e.Type = "cve"
			}
		}
		e.Mentions = count
		entities = append(entities, e)
	}
	return entities, nil
}

func (i *IntelligenceDB) GetArticlesByDate(date string, limit int) ([]models.Article, error) {
	rows, err := i.db.Query(`
		SELECT title, link, published_at, source_name, score, summary
		FROM articles
		WHERE substr(published_at, 1, 10) = ?
		ORDER BY score DESC, published_at DESC
		LIMIT ?
	`, date, limit)
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

func (i *IntelligenceDB) GetEntityGraph() ([]models.EntityNode, []models.EntityEdge, error) {
	nodeRows, err := i.db.Query(`
		SELECT e.name, e.type, COUNT(*) as mention_count
		FROM entities e
		JOIN article_entities ae ON e.name = ae.entity_name
		GROUP BY e.name, e.type
		ORDER BY mention_count DESC
	`)
	if err != nil {
		return nil, nil, err
	}
	defer nodeRows.Close()

	var nodes []models.EntityNode
	for nodeRows.Next() {
		var id, typ string
		var mentions int
		if err := nodeRows.Scan(&id, &typ, &mentions); err != nil {
			return nil, nil, err
		}
		if typ == "" || typ == "UNKNOWN" {
			if len(id) >= 4 && id[:4] == "CVE-" {
				typ = "cve"
			}
		}
		nodes = append(nodes, models.EntityNode{ID: id, Type: typ, Mentions: mentions})
	}

	edgeRows, err := i.db.Query(`
		SELECT ae1.entity_name, ae2.entity_name, COUNT(*) as weight
		FROM article_entities ae1
		JOIN article_entities ae2
			ON ae1.article_hash = ae2.article_hash
			AND ae1.entity_name < ae2.entity_name
		GROUP BY ae1.entity_name, ae2.entity_name
		ORDER BY weight DESC
	`)
	if err != nil {
		return nodes, nil, err
	}
	defer edgeRows.Close()

	var edges []models.EntityEdge
	for edgeRows.Next() {
		var src, dst string
		var weight int
		if err := edgeRows.Scan(&src, &dst, &weight); err != nil {
			return nodes, nil, err
		}
		edges = append(edges, models.EntityEdge{Source: src, Target: dst, Weight: weight})
	}

	return nodes, edges, nil
}

func (i *IntelligenceDB) GetArchiveDays() ([]ArchiveDay, error) {
	rows, err := i.db.Query(`
		SELECT substr(published_at, 1, 10) as d, COUNT(*) as c
		FROM articles
		GROUP BY d
		ORDER BY d DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var days []ArchiveDay
	for rows.Next() {
		var d ArchiveDay
		if err := rows.Scan(&d.Date, &d.Count); err != nil {
			return nil, err
		}
		days = append(days, d)
	}
	return days, nil
}

func (i *IntelligenceDB) GetArticlesByEntity(entityName string) ([]models.Article, error) {
	rows, err := i.db.Query(`
		SELECT a.title, a.link, a.published_at, a.source_name, a.score, a.summary
		FROM articles a
		JOIN article_entities ae ON a.hash = ae.article_hash
		WHERE ae.entity_name = ?
		ORDER BY a.published_at DESC
		LIMIT 20
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
