package main

import (
	connection_pooling_diff "andreashoj/deeper-learnings/internal/connection-pooling-diff"
	"andreashoj/deeper-learnings/internal/db"
	"fmt"
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
	fmt.Println(DB)

	connection_pooling_diff.LearningConnectionPooling(router)

	http.ListenAndServe(":8080", router)
}
