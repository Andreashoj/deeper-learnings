package main

import (
	"andreashoj/deeper-learnings/internal/db"
	transaction_isolation_levels "andreashoj/deeper-learnings/internal/transaction-isolation-levels"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func main() {
	DB, err := db.CreateDB()
	if err != nil {
		log.Fatalf("Failed starting DB: %s", err)
		return
	}

	router := chi.NewRouter()
	//fmt.Println(DB)

	//connection_pooling_diff.LearningConnectionPooling(router)
	transaction_isolation_levels.StartTransactionIsolationLevels(DB)

	http.ListenAndServe(":8080", router)
}
