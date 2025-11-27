package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func CreateDB() error {
	connString := "user=postgres host=localhost port=5432 dbname=test sslmode=disable"
	var err error
	DB, err = sql.Open("postgres", connString)

	if err != nil {
		return fmt.Errorf("failed creating database connection: %w", err)
	}

	err = DB.Ping()
	if err != nil {
		return fmt.Errorf("failed pinging database: %w", err)
	}

	DB.SetMaxOpenConns(20)
	DB.SetMaxIdleConns(5)

	return nil
}

func SeedDB(query string) {
	_, err := DB.Exec(query)
	if err != nil {
		log.Fatalf("failed seeding database: %s", err)
	}
}
