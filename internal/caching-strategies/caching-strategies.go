package caching_strategies

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisClient struct {
	client    *redis.Client
	hitCount  int
	missCount int
	ctx       context.Context
}

func StartCachingStrategies() {
	ctx := context.Background()
	rdb := startRedisClient()

	err := rdb.client.Ping(ctx).Err()
	if err != nil {
		fmt.Printf("Redis connection failed: %s\n", err)
		return
	}

	err = rdb.client.Set(ctx, "anz", "høj", 1000*time.Millisecond).Err()
	if err != nil {
		fmt.Printf("Failed setting value: %s", err)
		return
	}

	getUser(rdb)
	time.Sleep(1100 * time.Millisecond) // Exp 1s
	getUser(rdb)                        // should get nil

	rdb.GetHitRatio()
}

func getUser(rdb *redisClient) {
	val, err := rdb.client.Get(rdb.ctx, "anz").Result()
	if err != nil {
		rdb.recordMiss()
		fmt.Printf("Failed getting value: %s", err)
		return
	}

	rdb.recordHit()
	fmt.Println(val)
}

func startRedisClient() *redisClient {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})

	return &redisClient{
		client:    client,
		ctx:       context.Background(),
		missCount: 0,
		hitCount:  0,
	}
}

func (r *redisClient) recordHit() {
	r.hitCount++
}

func (r *redisClient) recordMiss() {
	r.missCount++
}

func (r *redisClient) GetHitRatio() {
	sum := r.missCount + r.hitCount
	fmt.Printf("\nCache hit ratio is: %v%%", float64(r.hitCount)/float64(sum)*100)
}
