package caching_strategies

import (
	"andreashoj/deeper-learnings/internal/db"
	query_profiling "andreashoj/deeper-learnings/internal/query-profiling"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

type testServer struct {
	server     *httptest.Server
	inMemCache map[string]interface{}
}

type loadBalancer struct {
	servers []*testServer
	redis   *redis.Client
	current int
}

var lb loadBalancer
var userID int

func StartRedisVsInMemory(r *chi.Mux) {
	lb = loadBalancer{
		redis: rdb,
	}

	r.Post("/api/rvm/user", logSpeed(lb.createRvmUser))
	r.Get("/api/rvm/user-mem", logSpeed(lb.getRvmUserInMem)) // It will only work every when the load balancer uses the server where the data was stored in
	// but it has a noticably big difference in query speed as it doesn't have to make any addiotnal network requests
	r.Get("/api/rvm/user-redis", logSpeed(lb.getRvmUserInRedis)) // Will work every time as redis is running in its own process

	servers := startTestServers(r)
	lb.servers = servers

	for _, s := range servers {
		fmt.Printf("\nserver running on: %s", s.server.URL)
	}
}

func logSpeed(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		start := time.Now()
		handlerFunc(writer, request)
		fmt.Println(time.Since(start))
	}
}

func startTestServers(r *chi.Mux) []*testServer {
	amount := 2
	var testServers []*testServer
	for i := 0; i < amount; i++ {
		server := testServer{
			server:     httptest.NewServer(r),
			inMemCache: make(map[string]interface{}),
		}
		testServers = append(testServers, &server)
	}

	return testServers
}

func (lb *loadBalancer) createRvmUser(w http.ResponseWriter, r *http.Request) {
	// Create user - cache user based off id in redis and lb server
	user := createUser()
	lb.getServer().cacheUserInMemory(user)
	lb.cacheUserInRedis(user, r.Context())
}

func (lb *loadBalancer) getRvmUserInMem(w http.ResponseWriter, r *http.Request) {
	// Get user
	userJSON := lb.getServer().getCache(fmt.Sprintf("user:%v", userID))
	if userJSON == nil {
		fmt.Printf("didn't find user")
		return
	}

	var user query_profiling.User
	err := json.Unmarshal(userJSON.([]byte), &user)
	if err != nil {
		fmt.Printf("failed decoding user")
		return
	}

	fmt.Println(user)
}

func (lb *loadBalancer) getRvmUserInRedis(w http.ResponseWriter, r *http.Request) {
	// Get user
	userJSON, err := rdb.Get(r.Context(), fmt.Sprintf("user:%v", userID)).Result()
	if err != nil {
		fmt.Sprintf("failed getting user: %s", err)
		return
	}

	var user query_profiling.User
	err = json.Unmarshal([]byte(userJSON), &user)

	if err != nil {
		fmt.Printf("failed decoding user: %s", err)
		return
	}

	fmt.Printf("Got user: %s", user)
}

func (lb *loadBalancer) getServer() *testServer {
	server := lb.servers[lb.current%len(lb.servers)]
	lb.current++
	if lb.current > len(lb.servers) {
		lb.current = 0
	}

	return server
}

func createUser() *query_profiling.User {
	user := query_profiling.User{
		Name:     "azn",
		Username: "doubleanz",
		Password: "secreeet uhh",
	}
	err := db.DB.QueryRow(`INSERT INTO users (name, username, password) VALUES ($1, $2, $3) RETURNING id`, user.Name, user.Username, user.Password).Scan(&user.Id)
	if err != nil {
		fmt.Printf("failed inserting user: %s", err)
		return nil
	}

	userID = user.Id // just for reference
	return &user
}

func (t *testServer) getCache(key string) interface{} {
	val, ok := t.inMemCache[key]
	if !ok {
		fmt.Printf("couldn't find cached user")
		return nil
	}

	return val
}

func (t *testServer) cacheUserInMemory(user *query_profiling.User) {
	userJSON, err := json.Marshal(user)
	if err != nil {
		fmt.Printf("failed saving user")
		return
	}
	fmt.Printf("saved as: %s", fmt.Sprintf("user:%v", user.Id))
	t.inMemCache[fmt.Sprintf("user:%v", user.Id)] = userJSON
}

func (lb *loadBalancer) cacheUserInRedis(user *query_profiling.User, ctx context.Context) {
	userJSON, err := json.Marshal(*user)
	if err != nil {
		fmt.Printf("failed saving user")
		return
	}
	fmt.Println(user)
	lb.redis.Set(ctx, fmt.Sprintf("user:%v", user.Id), userJSON, 1*time.Minute)
}
