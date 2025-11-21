package connection_pooling_diff

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func SeedDBS(pool, noPool *sql.DB) error {
	query, err := os.ReadFile("internal/connection-pooling-diff/init.sql")
	if err != nil {
		return fmt.Errorf("failed reading sql.init: %w", err)
	}

	_, err = pool.Exec(string(query))
	if err != nil {
		return fmt.Errorf("failed seeding pool db from sql file: %w", err)
	}

	_, err = noPool.Exec(string(query))
	if err != nil {
		return fmt.Errorf("failed seeding no pool from sql file: %w", err)
	}

	return nil
}

func StartDBWithoutPool() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "internal/connection-pooling-diff/no-pool.db")
	if err != nil {
		return nil, fmt.Errorf("failed starting no pool db: %w", err)
	}

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed pinging no pool db: %w", err)
	}

	db.SetMaxOpenConns(1)

	return db, nil
}

func StartDBWithPool(openConns int) (*sql.DB, error) {
	DB, err := sql.Open("sqlite3", "internal/connection-pooling-diff/pool.db")
	if err != nil {
		return nil, fmt.Errorf("failed starting no pool db: %w", err)
	}

	err = DB.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed pinging no pool db: %w", err)
	}

	DB.SetMaxOpenConns(openConns)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(5 * time.Minute)
	DB.SetConnMaxIdleTime(2 * time.Minute)

	return DB, nil
}
