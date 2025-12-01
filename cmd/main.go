package main

import (
	caching_strategies "andreashoj/deeper-learnings/internal/caching-strategies"
	"andreashoj/deeper-learnings/internal/helpers"
	"log"
	"net/http"

	"andreashoj/deeper-learnings/internal/db"

	"github.com/go-chi/chi/v5"
)

func main() {
	if err := db.CreateDB(); err != nil {
		log.Fatalf("Failed starting DB: %s", err)
		return
	}

	router := chi.NewRouter()
	helpers.NewCors(router)
	// fmt.Println(DB)

	// connection_pooling_diff.LearningConnectionPooling(router)
	// transaction_isolation_levels.StartTransactionIsolationLevels(DB)
	// transaction_deadlocks.StartTransactionDeadlock()
	// query_profiling.StartQueryProfiling()
	//db_replication.StartDBReplication(router)
	//caching_strategies.StartCachingStrategies()
	caching_strategies.StartCachingStrategiesHandler(router)

	http.ListenAndServe(":8080", router)
}
