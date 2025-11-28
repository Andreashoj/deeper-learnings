package db_replication

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

func StartDBReplication(r *chi.Mux) {
	pool := StartDatabasePool()

	fmt.Println(pool)
	RegisterEndpoints(r, pool)
}

func RegisterEndpoints(r *chi.Mux, pool *Pool) {
	// Writes
	r.Post("/api/user", func(w http.ResponseWriter, r *http.Request) {
		user := User{}
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			fmt.Printf("something went wrong: %s", err)
			w.WriteHeader(500)
			return
		}

		pool.CreateUser(user)
		updatedUser := pool.GetUser(user.Id) // Should be able to get the user, but it doesn't because the replica hasn't been updated at this point in time
		fmt.Println(updatedUser)

		time.Sleep(101 * time.Millisecond) // Wait for replicas to update
		updatedUser = pool.GetUser(user.Id)
		fmt.Printf("waited for replica to update: %s", updatedUser)
	})

	// Reads
	r.Get("/api/user", func(w http.ResponseWriter, r *http.Request) {
		replicaUser := pool.GetUser(1) // If the replica had been a real db, it would of course have been a query here
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(replicaUser.Name))
	})
}
