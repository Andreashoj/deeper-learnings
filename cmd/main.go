package main

import (
	"log"
	"net/http"

	"andreashoj/deeper-learnings/internal/db"
	db_replication "andreashoj/deeper-learnings/internal/db-replication"

	"github.com/go-chi/chi/v5"
)

func main() {
	if err := db.CreateDB(); err != nil {
		log.Fatalf("Failed starting DB: %s", err)
		return
	}

	router := chi.NewRouter()
	// fmt.Println(DB)

	// connection_pooling_diff.LearningConnectionPooling(router)
	// transaction_isolation_levels.StartTransactionIsolationLevels(DB)
	// transaction_deadlocks.StartTransactionDeadlock()
	// query_profiling.StartQueryProfiling()
	db_replication.StartDBReplication()

	http.ListenAndServe(":8080", router)
}
