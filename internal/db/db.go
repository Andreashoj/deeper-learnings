package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

func CreateDB() (*sql.DB, error) {
	connString := "user=postgres host=localhost port=5432 dbname=test sslmode=disable"
	db, err := sql.Open("postgres", connString)

	if err != nil {
		return nil, fmt.Errorf("failed creating database connection: %w", err)
	}

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed pinging database: %w", err)
	}

	return db, nil
}
