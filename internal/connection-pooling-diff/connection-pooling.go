package connection_pooling_diff

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

func LearningConnectionPooling(r *chi.Mux) {
	openConns := 100
	requestsToMake := 5000

	DBNoPool, err := StartDBWithoutPool()
	if err != nil {
		log.Fatalf("failed starting no pool db: %s", err)
		return
	}

	DBPool, err := StartDBWithPool(openConns)
	if err != nil {
		log.Fatalf("failed starting pool db: %s", err)
		return
	}

	if err = SeedDBS(DBPool, DBNoPool); err != nil {
		log.Fatalf("failed seedings dbs: %s", err)
		return
	}

	var wgPool sync.WaitGroup
	var wgNoPool sync.WaitGroup

	startPool := time.Now()
	for i := 0; i < requestsToMake; i++ {
		wgPool.Add(1)
		go func() {
			defer wgPool.Done()
			_, err = GetUsers(DBPool)
			if err != nil {
				log.Fatalf("faileding getting users from pool db: %s", err)
				return
			}

			// Could be used to track pool usage!
			//poolHealth(DBPool.Stats(), openConns)
		}()
	}
	wgPool.Wait()
	poolDuration := time.Since(startPool)

	startNoPool := time.Now()
	for i := 0; i < requestsToMake; i++ {
		wgNoPool.Add(1)
		go func() {
			defer wgNoPool.Done()
			_, err = GetUsers(DBNoPool)
			if err != nil {
				log.Fatalf("faileding getting users from no pool db: %s", err)
				return
			}

		}()
	}
	wgNoPool.Wait()
	noPoolDuration := time.Since(startNoPool)

	poolMs := float64(poolDuration.Milliseconds())
	noPoolMs := float64(noPoolDuration.Milliseconds())
	diff := ((noPoolMs - poolMs) / poolMs) * 100

	fmt.Printf("No pool spend %dms waiting for a connection to open up\n", DBNoPool.Stats().WaitDuration.Milliseconds()/int64(requestsToMake))
	fmt.Printf("Pool took: %v\n", poolDuration)
	fmt.Printf("No Pool took: %v\n", noPoolDuration)
	fmt.Printf("Pool performs %.2f%% better, with %v connections open compared to 1", diff, openConns)
}

func GetUsers(db *sql.DB) ([]User, error) {
	rows, err := db.Query(`SELECT id, username, email, updated_at, created_at FROM users`)
	if err != nil {
		return nil, fmt.Errorf("failed getting users: %w", err)
	}
	var users []User
	for rows.Next() {
		var user User
		err = rows.Scan(&user.ID, &user.Username, &user.Email, &user.UpdatedAt, &user.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed scanning user data: %w", err)
		}

		users = append(users, user)
	}

	return users, nil
}

func poolHealth(stats sql.DBStats, poolSize int) {
	if stats.WaitCount > 10000 {
		log.Println("WARNING: High wait count in pool!")
	}

	if stats.InUse == poolSize {
		log.Println("WARNING: Pool fully in use!")
	}
}

// Other strategies for ensuring a healthy connection pool and performance
// Short circut request if all connections are being used and then retry after n seconds
// Semaphore pattern to create queue with a max limit - and avoid starting unnecessary amounts of background jobs when there is no conn available anyways
