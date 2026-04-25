package main

import (
	"log"
	"news-cli/internal/database"
)

func main() {
	db, err := database.InitDB()
	if err != nil { log.Fatal(err) }
	
	// Delete noisy specific articles and clear out noisy generic Hacker News items
	queries := []string{
		`DELETE FROM articles WHERE title LIKE '%Squid%' OR title LIKE '%Cosmology%' OR title LIKE '%Examination%';`,
		`DELETE FROM articles WHERE source_name = 'HACKER NEWS FRONTPAGE' AND score < 10;`,
	}
	for _, q := range queries {
		_, err = db.GetDB().Exec(q)
		if err != nil { log.Println(err) }
	}
	
	// We'll also just delete ALL Hacker News Frontpage articles that are older than 2 hours to be safe
	_, _ = db.GetDB().Exec(`DELETE FROM articles WHERE source_name = 'HACKER NEWS FRONTPAGE' AND published_at < datetime('now', '-2 hours');`)
}
