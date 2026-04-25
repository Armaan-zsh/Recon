package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"path/filepath"
)

func main() {
	home, _ := os.UserHomeDir()
	db, err := sql.Open("sqlite3", filepath.Join(home, ".config/recon/nexus.db"))
	if err != nil { panic(err) }
	
	_, err = db.Exec("DELETE FROM articles WHERE title LIKE '%Squid%' OR title LIKE '%Cosmology%' OR title LIKE '%Examination%' OR source_name = 'HACKER NEWS FRONTPAGE'")
	if err != nil { fmt.Println(err) } else { fmt.Println("Cleaned!") }
}
