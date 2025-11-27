package transaction_deadlocks

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"andreashoj/deeper-learnings/internal/db"
)

type Account struct {
	ID      int
	Balance int
}

func StartTransactionDeadlock() {
	query := `
		DROP TABLE IF EXISTS accounts;
		CREATE TABLE accounts (
			id SERIAL PRIMARY KEY,
			balance INTEGER NOT NULL
		);
		INSERT INTO accounts (balance) VALUES (10), (20);
	`

	db.SeedDB(query)
	userA := 1
	userB := 2

	var wg sync.WaitGroup
	errsChan := make(chan error, 20)

	for range 100 {
		wg.Go(func() {
			from, to := userA, userB
			if rand.Intn(2) == 0 {
				from, to = userB, userA
			}
			// if err := deadlockIntroducingTransfer(userA, userB, 40); err != nil {
			if err := safeTransfer(from, to, 10); err != nil {
				errsChan <- err
			}
		})
	}

	go func() { // Nice little touch of adding the wait/close in routine so we can process the errors as they can streamed to the errsChan
		wg.Wait()
		close(errsChan)
	}()

	for err := range errsChan {
		fmt.Printf("something went wrong in the goroutine: %s", err)
	}
}

func deadlockIntroducingTransfer(fromID, toID int, amount int) error {
	ctx := context.Background()
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed starting transaction: %w", err)
	}
	defer tx.Rollback()

	toAccount := Account{ID: toID}
	fromAccount := Account{ID: fromID}
	// Cause deadlock by locking from in first job - that row is being used to in the other query causing a circular dependency
	err = tx.QueryRowContext(ctx, `SELECT balance FROM accounts WHERE id = $1  FOR UPDATE`, fromID).Scan(&fromAccount.Balance)
	if err != nil {
		return fmt.Errorf("failed getting fromAccount: %w", err)
	}

	time.Sleep(100 * time.Millisecond)

	err = tx.QueryRowContext(ctx, `SELECT balance FROM accounts WHERE id = $1 FOR UPDATE`, toID).Scan(&toAccount.Balance)
	if err != nil {
		return fmt.Errorf("failed getting toAccount: %w", err)
	}

	fromAccount.Balance -= amount
	toAccount.Balance += amount

	_, err = tx.ExecContext(ctx, `UPDATE accounts SET balance = $1 WHERE id = $2`, fromAccount.Balance, fromAccount.ID)
	if err != nil {
		return fmt.Errorf("failed updating from accounts balance: %w", err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE accounts SET balance = $1 WHERE id = $2`, toAccount.Balance, toAccount.ID)
	if err != nil {
		return fmt.Errorf("failed updating to accounts balance: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		fmt.Printf("failed commit transfer")
	}

	return nil
}

// Safe deadlock pattern implemented here, that ensures userID 1 cant end up waiting on user 2, while user waits on user 1
// Done by sorting the ID's here. Which means both queries tries to use row with userID 1 first, which is fine, because that row is released after first query is done
func safeTransfer(fromID, toID int, amount int) error {
	ctx := context.Background()
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed starting transaction: %w", err)
	}
	defer tx.Rollback()

	firstID := min(fromID, toID)
	secondID := max(fromID, toID)

	toAccount := Account{ID: toID}
	fromAccount := Account{ID: fromID}
	// Cause deadlock by locking from in first job - that row is being used to in the other query causing a circular dependency
	err = tx.QueryRowContext(ctx, `SELECT balance FROM accounts WHERE id = $1  FOR UPDATE`, firstID).Scan(&fromAccount.Balance)
	if err != nil {
		return fmt.Errorf("failed getting fromAccount: %w", err)
	}

	time.Sleep(100 * time.Millisecond)

	err = tx.QueryRowContext(ctx, `SELECT balance FROM accounts WHERE id = $1 FOR UPDATE`, secondID).Scan(&toAccount.Balance)
	if err != nil {
		return fmt.Errorf("failed getting toAccount: %w", err)
	}

	fromBalance := fromAccount.Balance
	toBalance := toAccount.Balance
	if firstID == toAccount.ID {
		toBalance = fromAccount.Balance
		fromBalance = toAccount.Balance
	}

	fromBalance -= amount
	toBalance += amount

	_, err = tx.ExecContext(ctx, `UPDATE accounts SET balance = $1 WHERE id = $2`, fromBalance, fromAccount.ID)
	if err != nil {
		return fmt.Errorf("failed updating from accounts balance: %w", err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE accounts SET balance = $1 WHERE id = $2`, toBalance, toAccount.ID)
	if err != nil {
		return fmt.Errorf("failed updating to accounts balance: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		fmt.Printf("failed commit transfer")
	}

	return nil
}
