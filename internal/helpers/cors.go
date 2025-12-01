package helpers

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func NewCors(r *chi.Mux) {
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://localhost:*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
}
