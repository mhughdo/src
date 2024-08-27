package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("localhost:%d", 6379),
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	fmt.Println("Connected to Redis")
	rdb.Set(context.Background(), "foo", "bar", 0).Err()
	wg := sync.WaitGroup{}
	wg.Add(4)
	go func() {
		defer wg.Done()
		res := rdb.XRead(context.Background(), &redis.XReadArgs{
			Streams: []string{"mystream12", "mystream13", "0", "0"},
			Block:   4 * time.Second,
		})
		fmt.Println("XRead", res.Val())
	}()
	go func() {
		defer wg.Done()
		time.Sleep(1 * time.Second)
		rdb.XAdd(context.Background(), &redis.XAddArgs{
			Stream: "mystream12",
			Values: map[string]interface{}{"foo": "bar"},
			ID:     "*",
		})
	}()
	go func() {
		defer wg.Done()
		time.Sleep(1 * time.Second)
		rdb.XAdd(context.Background(), &redis.XAddArgs{
			Stream: "mystream13",
			Values: map[string]interface{}{"foo": "bar"},
			ID:     "*",
		})
	}()
	go func() {
		defer wg.Done()
		val, err := rdb.Get(context.Background(), "foo").Result()
		if err != nil {
			panic(err)
		}
		fmt.Println("foo", val)
	}()
	wg.Wait()
}
