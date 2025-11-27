package transaction_isolation_levels

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

func StartTransactionIsolationLevels(DB *sql.DB) {
	err := StartSeed(DB)
	if err != nil {
		log.Fatalf("failed seeding db: %s", err)
		return
	}

	fmt.Println("Concurrent updates:") // Update balance concurrently and see lost update
	balanceToUpdate := 2

	// Notice business logic within increment balance is WRONG, because the updated value is lost in the concurrent functions
	go incrementBalance(DB, sql.LevelReadUncommitted, balanceToUpdate, 10)
	go incrementBalance(DB, sql.LevelReadUncommitted, balanceToUpdate, 10) // No fails, but update is lost because they both read the same concurrently value and then add the same amount to the same value
	go incrementBalance(DB, sql.LevelReadCommitted, balanceToUpdate, 10)
	go incrementBalance(DB, sql.LevelReadCommitted, balanceToUpdate, 10) // No fails, but update is lost because they both read the same concurrently value and then add the same amount to the same value

	time.Sleep(1000 * time.Millisecond)
	// High isolation levels here, brings awareness to the row, and sees another concurrent operation is affecting it and then locks it.
	// By default, in postgres these operations will fail, if they try to operate on a locked row
	go incrementBalance(DB, sql.LevelRepeatableRead, balanceToUpdate, 10) // Fails, because the row is locked from previous routine
	go incrementBalance(DB, sql.LevelSerializable, balanceToUpdate, 10)   // Fails, because the row is locked from previous routine

	// To avoid locked rows and failures on updates, implement retry logic to ensure correct data and isolation locked protected rows
	go incrementBalanceWithRetry(DB, sql.LevelRepeatableRead, balanceToUpdate, 10, 5) // Fails, because the row is locked from previous routine
	go incrementBalanceWithRetry(DB, sql.LevelSerializable, balanceToUpdate, 10, 5)   // Fails, because the row is locked from previous routine
}

type balance struct {
	Id     int
	Amount int
}

func incrementBalance(DB *sql.DB, isolationLevel sql.IsolationLevel, id int, amount int) error {
	ctx := context.Background()
	newBalance := balance{Id: id}
	tx, err := DB.BeginTx(ctx, &sql.TxOptions{
		Isolation: isolationLevel,
	})
	defer tx.Rollback()
	if err != nil {
		fmt.Printf("failed starting transaction: %s\n", err)
		return fmt.Errorf("failed starting transaction: %w", err)
	}

	currentBalance := 0
	err = tx.QueryRow(`SELECT amount FROM balances WHERE id = $1`, newBalance.Id).Scan(&currentBalance)
	if err != nil {
		log.Printf("failed getting updated balance: %s\n", err)
		return fmt.Errorf("failed updating balance: %w", err)
	}

	newBalance.Amount = currentBalance + amount
	time.Sleep(100 * time.Millisecond)

	_, err = tx.ExecContext(ctx, `UPDATE balances SET amount = $1 WHERE id = $2`, newBalance.Amount, newBalance.Id)
	if err != nil {
		log.Printf("failed creating new balance: %s\n", err)
		return fmt.Errorf("failed creating new balance: %w", err)
	}

	updatedBalance := balance{}
	err = tx.QueryRow(`SELECT id, amount FROM balances WHERE id = $1`, newBalance.Id).Scan(&updatedBalance.Id, &updatedBalance.Amount)
	if err != nil {
		log.Printf("failed creating new balance: %s\n", err)
		return fmt.Errorf("failed getting updated balance: %w", err)
	}

	// Business logic based off value
	if updatedBalance.Amount < 61 {
		fmt.Printf("Not much money on the bank account, only %v$, left! \n", updatedBalance.Amount)
	}

	if updatedBalance.Amount > 60 {
		fmt.Printf("Nice you got quite a lot money! All %v$, to your name! \n", updatedBalance.Amount)
	}

	if updatedBalance.Amount > 70 {
		fmt.Printf("DAMN, you balling! Retire now. You have %v$! \n", updatedBalance.Amount)
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("failed commiting transaction: %s\n", err)
		return fmt.Errorf("failed committing comitting transaction: %w", err)
	}

	return nil
}

func incrementBalanceWithRetry(DB *sql.DB, isolationLevel sql.IsolationLevel, id int, amount int, maxRetries int) error {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := incrementBalance(DB, isolationLevel, id, amount)
		if err == nil {
			return nil
		}

		fmt.Printf("Attempt %d failed, retrying...\n", attempt)
		time.Sleep(time.Duration(attempt*100) * time.Millisecond) // Backoff
	}

	return fmt.Errorf("failed after %d attempts", maxRetries)
}
