package server

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/app/server/config"
	"github.com/redis/go-redis/v9"
	"github.com/test-go/testify/assert"
	"github.com/test-go/testify/require"
)

func getAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	// Extract the port number from the address
	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

func startServer(ctx context.Context) (*Server, int, error) {
	port, err := getAvailablePort()
	if err != nil {
		return nil, 0, err
	}
	addr := fmt.Sprintf(":%d", port)
	cfg := config.NewConfig()
	err = cfg.Set(config.ListenAddrKey, addr)
	if err != nil {
		return nil, 0, err
	}
	srv := NewServer(cfg)
	go func() {
		if err := srv.Start(ctx); err != nil {
			panic(err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	return srv, port, nil
}

func TestGetAndSetCommand(t *testing.T) {
	ctx := context.Background()
	srv, port, err := startServer(ctx)
	require.NoError(t, err)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close(ctx)

	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("localhost:%d", port),
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	tests := []struct {
		name     string
		setup    func() error
		action   func() (interface{}, error)
		expected interface{}
		wantErr  bool
	}{
		{
			name: "Set and Get",
			setup: func() error {
				return rdb.Set(ctx, "foo", "bar", 0).Err()
			},
			action: func() (interface{}, error) {
				return rdb.Get(ctx, "foo").Result()
			},
			expected: "bar",
		},
		{
			name: "Set and Get: key expired",
			setup: func() error {
				return rdb.Set(ctx, "foo-expired", "bar", 100*time.Millisecond).Err()
			},
			action: func() (interface{}, error) {
				time.Sleep(200 * time.Millisecond)
				return rdb.Get(ctx, "foo-expired").Result()
			},
			expected: redis.Nil,
		},
		{
			name: "Set and Get with EXAT",
			setup: func() error {
				return rdb.SetArgs(ctx, "foo-exat", "bar", redis.SetArgs{ExpireAt: time.Now().Add(100 * time.Millisecond)}).Err()
			},
			action: func() (interface{}, error) {
				return rdb.Get(ctx, "foo-exat").Result()
			},
			expected: redis.Nil,
		},
		{
			name: "Set and Get with EXAT: key expired",
			setup: func() error {
				return rdb.SetArgs(ctx, "foo-exat-expired", "bar", redis.SetArgs{ExpireAt: time.Now().Add(100 * time.Millisecond)}).Err()
			},
			action: func() (interface{}, error) {
				time.Sleep(300 * time.Millisecond)
				return rdb.Get(ctx, "foo-exat-expired").Result()
			},
			expected: redis.Nil,
		},
		{
			name: "Get non-existent key",
			setup: func() error {
				return nil
			},
			action: func() (interface{}, error) {
				return rdb.Get(ctx, "non-existent-key").Result()
			},
			expected: redis.Nil,
		},
		{
			name: "Set NX - key does not exist",
			setup: func() error {
				return nil
			},
			action: func() (interface{}, error) {
				return rdb.SetArgs(ctx, "nx-key-does-not-exist", "bar", redis.SetArgs{Mode: "NX"}).Result()
			},
			expected: "OK",
		},
		{
			name: "Set NX - key exists",
			setup: func() error {
				return rdb.Set(ctx, "nx-key-exists", "bar", 0).Err()
			},
			action: func() (interface{}, error) {
				return rdb.SetArgs(ctx, "nx-key-exists", "bar", redis.SetArgs{Mode: "NX"}).Result()
			},
			expected: redis.Nil,
		},
		{
			name: "Set XX - key does not exist",
			setup: func() error {
				return nil
			},
			action: func() (interface{}, error) {
				return rdb.SetArgs(ctx, "xx-key-does-not-exist", "bar", redis.SetArgs{Mode: "XX"}).Result()
			},
			expected: redis.Nil,
		},
		{
			name: "Set XX - key exists",
			setup: func() error {
				return rdb.Set(ctx, "xx-key-exists", "bar", 0).Err()
			},
			action: func() (interface{}, error) {
				return rdb.SetArgs(ctx, "xx-key-exists", "bar", redis.SetArgs{Mode: "XX"}).Result()
			},
			expected: "OK",
		},
		{
			name: "Set with Get",
			setup: func() error {
				return rdb.Set(ctx, "set-with-get", "bar", 0).Err()
			},
			action: func() (interface{}, error) {
				return rdb.SetArgs(ctx, "set-with-get", "bar", redis.SetArgs{Get: true}).Result()
			},
			expected: "bar",
		},
		{
			name: "Set with Get - key not exist",
			setup: func() error {
				return nil
			},
			action: func() (interface{}, error) {
				return rdb.SetArgs(ctx, "set-with-get-not-exist", "bar", redis.SetArgs{Get: true}).Result()
			},
			expected: redis.Nil,
		},
		{
			name: "Set with Get and NX - key exists",
			setup: func() error {
				return rdb.Set(ctx, "set-with-get-nx-exists", "bar", 0).Err()
			},
			action: func() (interface{}, error) {
				return rdb.SetArgs(ctx, "set-with-get-nx-exists", "bar", redis.SetArgs{Mode: "NX", Get: true}).Result()
			},
			expected: "bar",
		},
		{
			name: "Set with Get and NX - key does not exist",
			setup: func() error {
				return nil
			},
			action: func() (interface{}, error) {
				return rdb.SetArgs(ctx, "set-with-get-nx-not-exist", "bar", redis.SetArgs{Mode: "NX", Get: true}).Result()
			},
			expected: redis.Nil,
		},
		{
			name: "Set with Get and XX - key exists",
			setup: func() error {
				return rdb.Set(ctx, "set-with-get-xx-exists", "bar", 0).Err()
			},
			action: func() (interface{}, error) {
				return rdb.SetArgs(ctx, "set-with-get-xx-exists", "bar", redis.SetArgs{Mode: "XX", Get: true}).Result()
			},
			expected: "bar",
		},
		{
			name: "Set with Get and XX - key does not exist",
			setup: func() error {
				return nil
			},
			action: func() (interface{}, error) {
				return rdb.SetArgs(ctx, "set-with-get-xx-not-exist", "bar", redis.SetArgs{Mode: "XX", Get: true}).Result()
			},
			expected: redis.Nil,
		},
		{
			name: "Set with KeepTTL",
			setup: func() error {
				return rdb.Set(ctx, "set-with-keepttl", "bar", 0).Err()
			},
			action: func() (interface{}, error) {
				return rdb.SetArgs(ctx, "set-with-keepttl", "baz", redis.SetArgs{KeepTTL: true}).Result()
			},
			expected: "OK",
		},
		{
			name: "Set with KeepTTL and EX",
			setup: func() error {
				return rdb.Set(ctx, "set-with-keepttl-nx", "bar", 10*time.Second).Err()
			},
			action: func() (interface{}, error) {
				return rdb.SetArgs(ctx, "set-with-keepttl-nx", "baz", redis.SetArgs{KeepTTL: true, ExpireAt: time.Now().Add(20 * time.Second)}).Result()
			},
			expected: "KEEPTTL option is not allowed with EX, PX, EXAT, or PXAT options",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				require.NoError(t, tt.setup())
			}
			result, err := tt.action()
			if tt.expected == nil {
				return
			}
			if tt.expected == redis.Nil {
				assert.Error(t, err)
				assert.Equal(t, redis.Nil, err)

			} else if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.expected, err.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestXReadCommand(t *testing.T) {
	ctx := context.Background()
	srv, port, err := startServer(ctx)
	require.NoError(t, err)
	defer srv.Close(ctx)

	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("localhost:%d", port),
		Password: "",
		DB:       0,
	})

	tests := []struct {
		name     string
		setup    func() error
		action   func() (interface{}, error)
		expected interface{}
		wantErr  bool
	}{
		{
			name: "XRead from empty stream",
			setup: func() error {
				return nil
			},
			action: func() (interface{}, error) {
				result, err := rdb.XRead(ctx, &redis.XReadArgs{
					Streams: []string{"mystream", "0-0"},
					Block:   -1,
				}).Result()
				if err == redis.Nil {
					return nil, nil
				}
				return result, err
			},
			expected: nil,
		},
		{
			name: "XRead with existing entries",
			setup: func() error {
				_, err := rdb.XAdd(ctx, &redis.XAddArgs{
					Stream: "mystream",
					ID:     "1-1",
					Values: map[string]interface{}{"field1": "value1"},
				}).Result()
				return err
			},
			action: func() (interface{}, error) {
				return rdb.XRead(ctx, &redis.XReadArgs{
					Streams: []string{"mystream", "0-0"},
					Block:   -1,
				}).Result()
			},
			expected: []redis.XStream{{Stream: "mystream", Messages: []redis.XMessage{{ID: "1-1", Values: map[string]interface{}{"field1": "value1"}}}}},
		},
		{
			name: "XRead with COUNT",
			setup: func() error {
				for i := 0; i < 5; i++ {
					_, err := rdb.XAdd(ctx, &redis.XAddArgs{
						Stream: "mystream2",
						ID:     "*",
						Values: map[string]interface{}{fmt.Sprintf("field%d", i): fmt.Sprintf("value%d", i)},
					}).Result()
					if err != nil {
						return err
					}
				}
				return nil
			},
			action: func() (interface{}, error) {
				streams, err := rdb.XRead(ctx, &redis.XReadArgs{
					Streams: []string{"mystream2", "0-0"},
					Count:   3,
					Block:   -1,
				}).Result()
				if err != nil {
					return nil, err
				}
				return len(streams[0].Messages), nil
			},
			expected: 3,
		},
		{
			name: "XRead with BLOCK (timeout)",
			setup: func() error {
				return nil
			},
			action: func() (interface{}, error) {
				start := time.Now()
				_, err := rdb.XRead(ctx, &redis.XReadArgs{
					Streams: []string{"mystream3", "$"},
					Block:   time.Second,
				}).Result()
				duration := time.Since(start)
				if err == redis.Nil {
					return duration >= time.Second, nil
				}
				return nil, err
			},
			expected: true,
		},
		{
			name: "XRead with BLOCK (new entry added)",
			setup: func() error {
				return nil
			},
			action: func() (interface{}, error) {
				go func() {
					time.Sleep(500 * time.Millisecond)
					_, err := rdb.XAdd(ctx, &redis.XAddArgs{
						Stream: "mystream6",
						ID:     "1-1",
						Values: map[string]interface{}{"field1": "value1"},
					}).Result()
					if err != nil {
						t.Errorf("Failed to add entry: %v", err)
					}
				}()

				return rdb.XRead(ctx, &redis.XReadArgs{
					Streams: []string{"mystream6", "$"},
					Block:   2 * time.Second,
				}).Result()
			},
			expected: []redis.XStream{{Stream: "mystream6", Messages: []redis.XMessage{{ID: "1-1", Values: map[string]interface{}{"field1": "value1"}}}}},
		},
		{
			name: "XRead with BLOCK (multiple streams)",
			setup: func() error {
				return nil
			},
			action: func() (interface{}, error) {
				go func() {
					time.Sleep(500 * time.Millisecond)
					_, err := rdb.XAdd(ctx, &redis.XAddArgs{
						Stream: "mystream7",
						ID:     "1-1",
						Values: map[string]interface{}{"field1": "value1"},
					}).Result()
					if err != nil {
						t.Errorf("Failed to add entry: %v", err)
					}
				}()

				return rdb.XRead(ctx, &redis.XReadArgs{
					Streams: []string{"mystream7", "mystream8", "$", "$"},
					Block:   2 * time.Second,
				}).Result()
			},
			expected: []redis.XStream{{Stream: "mystream7", Messages: []redis.XMessage{{ID: "1-1", Values: map[string]interface{}{"field1": "value1"}}}}},
		},
		{
			name: "XRead (multiple streams and multiple entries)",
			setup: func() error {
				_, err := rdb.XAdd(ctx, &redis.XAddArgs{
					Stream: "mystream20",
					ID:     "1-1",
					Values: map[string]interface{}{"field1": "value1"},
				}).Result()
				if err != nil {
					return err
				}
				_, err = rdb.XAdd(ctx, &redis.XAddArgs{
					Stream: "mystream21",
					ID:     "2-2",
					Values: map[string]interface{}{"field2": "value2"},
				}).Result()
				if err != nil {
					return err
				}
				_, err = rdb.XAdd(ctx, &redis.XAddArgs{
					Stream: "mystream20",
					ID:     "1-2",
					Values: map[string]interface{}{"field1": "value1"},
				}).Result()
				if err != nil {
					return err
				}
				_, err = rdb.XAdd(ctx, &redis.XAddArgs{
					Stream: "mystream21",
					ID:     "2-3",
					Values: map[string]interface{}{"field2": "value2"},
				}).Result()
				if err != nil {
					return err
				}
				return nil
			},
			action: func() (interface{}, error) {
				return rdb.XRead(ctx, &redis.XReadArgs{
					Streams: []string{"mystream20", "mystream21", "0-0", "0-0"},
					Block:   -1,
				}).Result()
			},
			expected: []redis.XStream{{Stream: "mystream20", Messages: []redis.XMessage{
				{ID: "1-1", Values: map[string]interface{}{"field1": "value1"}},
				{ID: "1-2", Values: map[string]interface{}{"field1": "value1"}},
			}},
				{
					Stream: "mystream21", Messages: []redis.XMessage{
						{ID: "2-2", Values: map[string]interface{}{"field2": "value2"}},
						{ID: "2-3", Values: map[string]interface{}{"field2": "value2"}},
					},
				},
			},
		},
		{
			name: "XRead with BLOCK and COUNT",
			setup: func() error {
				return nil
			},
			action: func() (interface{}, error) {
				go func() {
					for i := 0; i < 3; i++ {
						_, err := rdb.XAdd(ctx, &redis.XAddArgs{
							Stream: "mystream9",
							ID:     "*",
							Values: map[string]interface{}{fmt.Sprintf("field%d", i): fmt.Sprintf("value%d", i)},
						}).Result()
						if err != nil {
							t.Errorf("Failed to add entry: %v", err)
						}
					}
				}()

				streams, err := rdb.XRead(ctx, &redis.XReadArgs{
					Streams: []string{"mystream9", "0-0"},
					Block:   2 * time.Second,
					Count:   2,
				}).Result()
				if err != nil {
					return nil, err
				}
				return len(streams[0].Messages), nil
			},
			expected: 1,
		},
		{
			name: "XRead with BLOCK (immediate return for non-empty stream)",
			setup: func() error {
				_, err := rdb.XAdd(ctx, &redis.XAddArgs{
					Stream: "mystream10",
					ID:     "*",
					Values: map[string]interface{}{"field1": "value1"},
				}).Result()
				return err
			},
			action: func() (interface{}, error) {
				start := time.Now()
				_, err := rdb.XRead(ctx, &redis.XReadArgs{
					Streams: []string{"mystream10", "+"},
					Block:   5 * time.Second,
				}).Result()
				duration := time.Since(start)
				if err != nil {
					return nil, err
				}
				return duration < time.Second, nil
			},
			expected: true,
		},
		{
			name: "XRead with multiple streams",
			setup: func() error {
				_, err := rdb.XAdd(ctx, &redis.XAddArgs{
					Stream: "mystream4",
					ID:     "*",
					Values: map[string]interface{}{"field1": "value1"},
				}).Result()
				if err != nil {
					return err
				}
				_, err = rdb.XAdd(ctx, &redis.XAddArgs{
					Stream: "mystream5",
					ID:     "*",
					Values: map[string]interface{}{"field2": "value2"},
				}).Result()
				return err
			},
			action: func() (interface{}, error) {
				streams, err := rdb.XRead(ctx, &redis.XReadArgs{
					Streams: []string{"mystream4", "mystream5", "0-0", "0-0"},
					Block:   -1,
				}).Result()
				if err != nil {
					return nil, err
				}
				return len(streams), nil
			},
			expected: 2, // Number of streams
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				require.NoError(t, tt.setup())
			}
			result, err := tt.action()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
