package caching_strategies

import (
	"andreashoj/deeper-learnings/internal/db"
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

	r.Post("/api/user-invalidate", handlerAnalyzer(createUserManualCacheInvalidation))
	r.Post("/api/user-update", handlerAnalyzer(createUserCacheUpdate))
}

func handlerAnalyzer(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		handler(w, r)
		duration := time.Since(now)
		fmt.Printf("\nhandler took: %v", duration)
	}
}

// Could be abstracted into it's own package with accessors - "cache" => GetUserKey, GetUsersKey?
const (
	CacheKeyUsers = "users"
)

func createUserManualCacheInvalidation(w http.ResponseWriter, r *http.Request) {
	// Manually invalidate entire users cache - fast and efficient depending on how often users are created..
	// If running into performance issues - alternative approach => createUserCacheUpdate
	user, err := repoCreateUser(r)
	if err != nil {
		fmt.Printf("failed getting user from request: %s", err)
		return
	}

	err = rdb.Del(r.Context(), CacheKeyUsers).Err()
	if err != nil {
		fmt.Printf("failed deleting user cache: %s", err)
		return
	}

	response, err := json.Marshal(user)
	if err != nil {
		fmt.Sprintf("failed converting user to json: %s", err)
		return
	}

	w.WriteHeader(201)
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func createUserCacheUpdate(w http.ResponseWriter, r *http.Request) {
	user, err := repoCreateUser(r)
	if err != nil {
		fmt.Printf("failed getting user from request: %s", err)
		return
	}

	cachedUsers, err := rdb.Get(r.Context(), CacheKeyUsers).Result()
	if err != nil { // No cache found
		fmt.Printf("didn't get users from cache: %s", err)
		// Create
		rows, err := db.DB.Query("SELECT id, name FROM users")
		if err != nil {
			fmt.Printf("failed getting users: %s", err)
			return
		}

		var users []query_profiling.User
		for rows.Next() {
			var u query_profiling.User
			if err = rows.Scan(&u.Id, &u.Name); err != nil {
				fmt.Printf("failed decoding user: %s", err)
				return
			}

			users = append(users, u)
		}

		response, err := json.Marshal(users)
		rdb.Set(r.Context(), CacheKeyUsers, response, 5*time.Minute)
		if err != nil {
			fmt.Printf("failed encoding users: %s", err)
			return
		}

		w.WriteHeader(201)
		w.Header().Set("Content-Type", "application/json")
		w.Write(response)
		return
	}

	var users []query_profiling.User
	err = json.Unmarshal([]byte(cachedUsers), &users)
	if err != nil {
		fmt.Printf("failed decoding users: %s", err)
		return
	}

	users = append(users, *user)
	response, err := json.Marshal(users)
	if err != nil {
		fmt.Printf("failed creating response: %s", err)
		return
	}

	err = rdb.Set(r.Context(), CacheKeyUsers, response, 5*time.Minute).Err()
	if err != nil {
		fmt.Printf("failed setting users cache: %s", err)
		return
	}

	w.WriteHeader(201)
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func getUsers(w http.ResponseWriter, r *http.Request) {
	// get request details - none in this case kinda
	// create cache key based off request details

	// try cache
	result, err := rdb.Get(r.Context(), CacheKeyUsers).Result()
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
	rdb.Set(r.Context(), CacheKeyUsers, usersJSON, 5*time.Minute)

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

func repoCreateUser(r *http.Request) (*query_profiling.User, error) {
	var user query_profiling.User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		fmt.Printf("failed decoding user: %s", err)
		return nil, err
	}

	err = db.DB.QueryRow("INSERT INTO users (name) VALUES ($1) RETURNING id", user.Name).Scan(&user.Id)
	if err != nil {
		fmt.Printf("failed inserting user: %s", err)
		return nil, err
	}

	return &user, nil
}
