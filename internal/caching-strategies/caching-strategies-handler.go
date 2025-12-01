package caching_strategies

import (
	query_profiling "andreashoj/deeper-learnings/internal/query-profiling"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

var rdb = redis.NewClient(&redis.Options{
	Addr:     "localhost:6380",
	Password: "",
	DB:       0,
})

func StartCachingStrategiesHandler(r *chi.Mux) {
	query_profiling.InsertUsersAndPosts() // Seed DB with users and posts

	r.Get("/api/cache/hit", handlerAnalyzer(getUsers))
	r.Get("/api/no-cache/hit", handlerAnalyzer(getUsersNoCache))
	r.Get("/api/cache/posts", handlerAnalyzer(getUsersAndPosts))
	r.Get("/api/no-cache/posts", handlerAnalyzer(getUsersAndPostsNoCache))
}

func handlerAnalyzer(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		handler(w, r)
		duration := time.Since(now)
		fmt.Printf("\nhandler took: %v", duration)
	}
}

func getUsers(w http.ResponseWriter, r *http.Request) {
	// get request details - none in this case kinda
	// create cache key based off request details
	cacheKey := "users"

	// try cache
	result, err := rdb.Get(r.Context(), cacheKey).Result()
	if err != nil {
		fmt.Printf("didnt find cached content: %s", err)
	}

	if err == nil {
		var users []query_profiling.User
		err = json.Unmarshal([]byte(result), &users)
		if err != nil {
			fmt.Printf("failed converting json to users slice: %s", err)
			return
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(users)
		return
	}

	// get from db
	users, err := query_profiling.GetUsers()
	if err != nil {
		fmt.Printf("failed getting users: %s", err)
		return
	}
	usersJSON, err := json.Marshal(users)
	if err != nil {
		fmt.Printf("failed converting users to json: %s", err)
		return
	}

	// store cache
	rdb.Set(r.Context(), cacheKey, usersJSON, 5*time.Minute)

	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	w.Write(usersJSON)
}

func getUsersNoCache(w http.ResponseWriter, r *http.Request) {
	// get from db
	users, err := query_profiling.GetUsers()
	if err != nil {
		fmt.Printf("failed getting users: %s", err)
		return
	}
	usersJSON, err := json.Marshal(users)
	if err != nil {
		fmt.Printf("failed converting users to json: %s", err)
		return
	}

	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	w.Write(usersJSON)
}

func getUsersAndPosts(w http.ResponseWriter, r *http.Request) {
	cacheKey := "posts_and_users"

	result, err := rdb.Get(r.Context(), cacheKey).Result()
	if err == nil {
		w.WriteHeader(200)
		w.Write([]byte(result))
	}

	posts, users, err := query_profiling.GetPostsAndUsersNPlus()
	if err != nil {
		fmt.Printf("failed getting posts and users: %s", err)
		return
	}

	response := map[string]interface{}{
		"users": users,
		"posts": posts,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		fmt.Printf("failed converting response to json: %s", err)
		return
	}

	rdb.Set(r.Context(), cacheKey, responseJSON, 5*time.Minute)

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
	w.WriteHeader(200)
}

func getUsersAndPostsNoCache(w http.ResponseWriter, r *http.Request) {
	posts, users, err := query_profiling.GetPostsAndUsersNPlus()
	if err != nil {
		fmt.Printf("failed getting posts and users: %s", err)
		return
	}

	response := map[string]interface{}{
		"users": users,
		"posts": posts,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		fmt.Printf("failed converting response to json: %s", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
	w.WriteHeader(200)
}
