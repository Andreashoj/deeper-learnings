package transaction_isolation_levels

import (
	"database/sql"
	"fmt"
	"os"
)

func StartSeed(DB *sql.DB) error {
	query, err := os.ReadFile("internal/transaction-isolation-levels/seed.sql")
	if err != nil {
		return fmt.Errorf("failed reading the seed.sql file: %w", err)
	}

	_, err = DB.Exec(string(query))
	if err != nil {
		return fmt.Errorf("failed executing sql query: %w", err)
	}

	return nil
}
