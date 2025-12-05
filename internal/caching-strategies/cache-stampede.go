package caching_strategies

import (
	query_profiling "andreashoj/deeper-learnings/internal/query-profiling"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

var RDB = redis.NewClient(&redis.Options{
	Addr:     "localhost:6380",
	Password: "",
	DB:       0,
})

var TTL = 5 * time.Minute

func StartCacheStampedeDemo(r *chi.Mux) {
	// Problem: /dashboard (api/post, api/user, api/stats) is being hit by 1000 requests concurrently, and the cache JUST expired!
	// How do we handle this and how could it be prevented?

	// Jitter TTL [X]
	// Refresh cache in the background when endpoint sees exp is close [X]
	// Refresh worker
	// Mutex lock on cache [X]
	// Event driven - when to update cache ?

	registerDashboardEndpoints(r)
	ts := httptest.NewServer(r)
	stampedeApiWithRequests(ts)
}

func registerDashboardEndpoints(r *chi.Mux) {
	r.Get("/api/dashboard/post", getPosts)
	r.Get("/api/dashboard/post-mutex", getPostsWithMutex)
	r.Get("/api/dashboard/post-event-driven", getPostsWithEventDriven)

	startCachingWorkerPosts(context.Background(), "posts")
	r.Get("/api/dashboard/post-worker", getPostsWithWorker)
}

func stampedeApiWithRequests(server *httptest.Server) {
	for i := 0; i < 1000; i++ {
		endpoint := []string{"post", "user", "stats"}[rand.Intn(2)]
		go func() {
			_, err := http.Get(server.URL + fmt.Sprintf("/api/dashboard/%s", endpoint))
			if err != nil {
				fmt.Printf("failed getting endpoint: %s", err)

				return
			}
		}()
	}
}

// First issue here is that we have a set TTL of 5 minutes, which means that all our endpoints cache all runs out at the same time
// The strategy here is use jittered TTL, which means TTL is randomized to some extent => 5 minutes + random seconds
// The point is to avoid all the caching expiring at the same time, and by that offloading the database a bit, by maybe only having to request posts and users, instead of posts, users and stats
func getJitteredTTL() time.Duration {
	randSecs := time.Duration(rand.Intn(60)) * time.Second
	return TTL + randSecs
}

func getPosts(w http.ResponseWriter, r *http.Request) {
	cacheKeyPosts := "posts"
	_, err := RDB.Get(r.Context(), cacheKeyPosts).Result()
	if err == nil { // Got cache
		fmt.Printf("success! Got posts cache")

		// We check here to see if the cache expiration is low, if so, we refresh the cache in the background
		exp := RDB.TTL(r.Context(), cacheKeyPosts).Val()
		if exp < (1 * time.Minute) {
			go refreshPostsCache(r.Context(), cacheKeyPosts)
		}

		return
	}

	fmt.Printf("No cache, queried posts from DB")
	refreshPostsCache(r.Context(), cacheKeyPosts)
}

func refreshPostsCache(ctx context.Context, cacheKeyPosts string) {
	posts, err := query_profiling.GetPosts()
	if err != nil {
		fmt.Printf("failed getting posts: %s", err)
		return
	}

	postsJSON, err := json.Marshal(posts)
	if err != nil {
		fmt.Printf("failed decoding posts to json: %s", postsJSON)
		return
	}

	RDB.Set(ctx, cacheKeyPosts, postsJSON, getJitteredTTL())
	fmt.Print("Updated posts cache")
}

var cacheLocks = make(map[string]*sync.Mutex)
var mu sync.Mutex

func getPostsWithMutex(w http.ResponseWriter, r *http.Request) {
	cacheKey := "posts"
	res, err := RDB.Get(r.Context(), cacheKey).Result()
	if err == nil { // Cache was found
		var posts []query_profiling.Post
		err = json.Unmarshal([]byte(res), &posts)
		if err != nil {
			fmt.Printf("couldn't decode cached posts: %s", err)
			return
		}

		fmt.Printf("Got cached posts!")
		return
	}

	mu.Lock()
	if _, exists := cacheLocks[cacheKey]; !exists {
		cacheLocks[cacheKey] = &sync.Mutex{}
	}
	lock := cacheLocks[cacheKey]
	mu.Unlock()

	lock.Lock()
	defer lock.Unlock()
	// race condition the first try of getting the cache and this line, the cache may have been set from a
	// concurrent running getPostsWithMutex function, so try again here - if that fails, it has not been set, and this function will then do it
	_, err = RDB.Get(r.Context(), cacheKey).Result()
	if err == nil {
		fmt.Printf("Got cached posts, after running the lock")
		return
	}

	posts, err := query_profiling.GetPosts()
	if err != nil {
		fmt.Printf("failed getting posts: %s", err)
		return
	}

	postsJSON, err := json.Marshal(posts)
	if err != nil {
		fmt.Printf("failed decoding posts: %s", err)
		return
	}

	RDB.Set(r.Context(), cacheKey, postsJSON, getJitteredTTL())
	fmt.Printf("Set posts cache")
	return
}

// This is a specific example of a worker only updating 1 cache key - it could be also have been made with a generic approach, that updates all the cache keys in the cache client
func startCachingWorkerPosts(ctx context.Context, cacheKey string) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("stopped cache worker")
			return
		case <-ticker.C:
			_, err := RDB.Get(ctx, cacheKey).Result()
			if err == nil { // Got cache
				fmt.Printf("success! Got posts cache")

				// We check here to see if the cache expiration is low, if so, we refresh the cache in the background
				exp := RDB.TTL(ctx, cacheKey).Val()
				if exp < (1 * time.Minute) {
					refreshPostsCache(ctx, cacheKey)
				}

			}

			refreshPostsCache(ctx, cacheKey)
		}
	}
}

func getPostsWithWorker(w http.ResponseWriter, r *http.Request) {
	cacheKeyPosts := "posts"
	_, err := RDB.Get(r.Context(), cacheKeyPosts).Result()
	if err == nil {
		fmt.Printf("Got expected posts data from cache")
		return
	}

	_, err = query_profiling.GetPosts()
	if err != nil {
		fmt.Printf("failed getting posts: %s", err)
		return
	}

	fmt.Printf("No cache, queried posts from DB - the worker didn't do it's job")
}

func getPostsWithEventDriven(w http.ResponseWriter, r *http.Request) {
	// Event driven is often used if data must NEVER be served stale, but the data doesn't get changed often enough that the trade off of not using cache is worth it
	_, err := query_profiling.CreatePost("my post", 1)
	if err != nil {
		fmt.Printf("failed creating post: %s", err)
		return
	}

	// Cache serves fresh data
	refreshPostsCache(r.Context(), "posts")
}
