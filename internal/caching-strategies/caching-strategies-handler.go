package caching_strategies

import (
	"andreashoj/deeper-learnings/internal/db"
	query_profiling "andreashoj/deeper-learnings/internal/query-profiling"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

var rdb = redis.NewClient(&redis.Options{
	Addr:     "localhost:6380",
	Password: "",
	DB:       0,
})

// Could be abstracted into it's own package with accessors - "cache" => GetUserKey, GetUsersKey?
const (
	CacheKeyUsers = "users"
	CacheKeyUser  = "users:%v"
)

func StartCachingStrategiesHandler(r *chi.Mux) {
	query_profiling.InsertUsersAndPosts() // Seed DB with users and posts

	r.Get("/api/cache/hit", handlerAnalyzer(getUsers))
	r.Get("/api/no-cache/hit", handlerAnalyzer(getUsersNoCache))
	r.Get("/api/cache/posts", handlerAnalyzer(getUsersAndPosts))
	r.Get("/api/no-cache/posts", handlerAnalyzer(getUsersAndPostsNoCache))

	r.Post("/api/user-invalidate", handlerAnalyzer(createUserManualCacheInvalidation))
	r.Post("/api/user-update", handlerAnalyzer(createUserCacheUpdate))

	// Demonstrate stale data security risk case -
	// User logs in and that user is then cached

	updateUserRole(r)
}

func handlerAnalyzer(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		handler(w, r)
		duration := time.Since(now)
		fmt.Printf("\nhandler took: %v", duration)
	}
}

func updateUserRole(r *chi.Mux) {
	// Register routes
	// Log user in - cache user
	// Update user role - admin updates the user
	// Make "unauthorized" request from user - that will succeed wrongfully

	userID := 1
	userAdminRole := 999

	r.Post("/api/login", login)
	r.Post("/api/user", updateUserPermissions)
	r.Get("/api/secret-data", getUserDetails)

	testServer := httptest.NewServer(r)
	defer testServer.Close()

	// For set up, set the users permissions to be 999 - which is the required permission id for getting the secret data
	_, err := db.DB.Exec("INSERT INTO users_permissions (user_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING ", userID, userAdminRole)
	if err != nil {
		fmt.Printf("failed inserting the user / permission role into the users_permissions table: %s", err)
		return
	}

	// Simulate bad path here
	// Login req - caches user
	loginRes, err := http.Post(
		testServer.URL+"/api/login",
		"application/json",
		bytes.NewBufferString(`{"username": "anz", "password": "tester12"}`),
	)

	if err != nil {
		fmt.Printf("failed logging in..: %s", err)
		return
	}

	defer loginRes.Body.Close()

	body, err := io.ReadAll(loginRes.Body)
	if err != nil {
		fmt.Printf("failed reading login response body: %s", err)
		return
	}

	fmt.Println(string(body)) // User!

	// Update user permission
	updateUserPermissionResp, err := http.Post(
		testServer.URL+"/api/user",
		"application/json",
		bytes.NewBufferString(`{ "id": 1, "permissions": [1, 2]}`),
	)

	if err != nil {
		fmt.Printf("failed updating user permissions: %s", err)
		return
	}

	defer updateUserPermissionResp.Body.Close()

	// Make request that requires ADMIN - (999)
	getUsersRes, err := http.Get(testServer.URL + "/api/secret-data")

	if err != nil {
		fmt.Printf("failed logging in..: %s", err)
		return
	}

	defer loginRes.Body.Close()

	body, err = io.ReadAll(getUsersRes.Body)
	if err != nil {
		fmt.Printf("failed getting user res body: %s", err)
		return
	}
	fmt.Println(body)
}

type UserRes struct {
	UserID      int    `json:"user_id,omitempty"`
	Username    string `json:"username,omitempty"`
	Permissions []int  `json:"permissions,omitempty"`
}

func login(w http.ResponseWriter, r *http.Request) {
	userID := 1
	// Authorize user
	var req struct {
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
	}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		fmt.Printf("failed decoding user: %s", err)
		return
	}

	var res UserRes
	res.UserID = userID

	rows, err := db.DB.Query(`SELECT users.name, permission_id FROM users LEFT JOIN users_permissions ON users.id = users_permissions.user_id WHERE id = $1 `, userID)
	if err != nil {
		fmt.Printf("failed getting user: %s", err)
		return
	}

	var permissions []int
	for rows.Next() {
		var permission int
		err = rows.Scan(&res.Username, &permission)
		if err != nil {
			fmt.Printf("failed mapping username/permissions: %s", err)
			return
		}

		permissions = append(permissions, permission)
	}

	res.Permissions = permissions

	response, err := json.Marshal(res)
	if err != nil {
		fmt.Printf("failed decoding user: %s", err)
		return
	}

	// Cache user
	rdb.Set(r.Context(), fmt.Sprintf(CacheKeyUser, res.UserID), response, 5*time.Minute)

	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func updateUserPermissions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Id          int   `json:"id,omitempty"`
		Permissions []int `json:"permissions,omitempty"`
	}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		fmt.Printf("failed decoding user: %s", err)
		return
	}

	// Start transaction..
	tx, err := db.DB.BeginTx(r.Context(), nil)
	if err != nil {
		fmt.Printf("failed starting transaction: %s", err)
		return
	}
	defer tx.Rollback()

	// Delete
	_, err = tx.ExecContext(r.Context(), `DELETE FROM users_permissions WHERE user_id = $1`, req.Id)
	if err != nil {
		fmt.Printf("failed deleting user permission: %s", err)
		return
	}

	// Create new
	_, err = tx.Exec("INSERT INTO users_permissions (user_id, permission_id) SELECT $1 ja, unnest($2::int[])", req.Id, pq.Array(req.Permissions))
	if err != nil {
		fmt.Printf("failed inserting users and permissions")
		return
	}

	// NOT updating the cached user here.
	err = tx.Commit()
	if err != nil {
		fmt.Printf("failed commiting transaction: %s", err)
		return
	}

	fmt.Printf("updated user the user to have permissions: %v", req.Permissions)
	w.WriteHeader(201)
}

func getUserDetails(w http.ResponseWriter, r *http.Request) {
	// Usually the id retrieval and authorization would happen through a guard and cookie etc, we don't need that for testing though! So the id is just the same as in the previous endpoints
	userID := 1
	adminID := 999
	cachedUserWithPermissions, err := rdb.Get(r.Context(), fmt.Sprintf(CacheKeyUser, userID)).Result()
	if err != nil {
		fmt.Printf("couldn't find cached user: %s", err)
		return
	}

	var user UserRes
	err = json.Unmarshal([]byte(cachedUserWithPermissions), &user)
	if err != nil {
		fmt.Printf("failed decoding cached user: %s", err)
		return
	}

	// At this point the user does NOT have admin privileges, it was removed in the previous request
	hasAdmin := false
	for _, permission := range user.Permissions {
		if permission == adminID { // This becomes true because the user.Permissions is from the cache which is STALE and did not get updated in the previous request
			hasAdmin = true
			break
		}
	}

	if hasAdmin {
		// Get details of all users
		var users []query_profiling.User
		rows, err := db.DB.Query(`SELECT id, name, username FROM users`)
		if err != nil {
			fmt.Printf("failed getting users: %s", err)
			return
		}
		for rows.Next() {
			var user query_profiling.User
			err = rows.Scan(&user.Id, &user.Name, &user.Username)
			if err != nil {
				fmt.Printf("failed mapping user: %s", err)
				return
			}

			users = append(users, user)
		}

		fmt.Println("here")
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
		return
	}

	w.WriteHeader(401)
}

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
