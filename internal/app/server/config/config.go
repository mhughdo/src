package config

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

const (
	ListenAddrKey = "LISTENADDR"
	DirKey        = "DIR"
	DBFilenameKey = "DBFILENAME"
	ReplicaOfKey  = "REPLICAOF"
)

var (
	WriteableCommands = map[string]struct{}{
		"set":  {},
		"del":  {},
		"incr": {},
	}
)

var supportedOptions = map[string]struct{}{
	ListenAddrKey: {},
	DirKey:        {},
	DBFilenameKey: {},
	ReplicaOfKey:  {},
}

type Config struct {
	mu      sync.Mutex
	options map[string]string
}

var (
	ErrOptionNotFound = errors.New("unknown option")
	ErrInvalidSetting = errors.New("invalid option")
)

func NewConfig() *Config {
	return &Config{
		options: make(map[string]string),
	}
}

func (c *Config) Set(key, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := supportedOptions[strings.ToUpper(key)]; !exists {
		return fmt.Errorf("%s for %s", ErrOptionNotFound, key)
	}
	c.options[key] = value
	return nil
}

func (c *Config) SetBatch(pairs map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, value := range pairs {
		if _, exists := supportedOptions[strings.ToUpper(key)]; !exists {
			return fmt.Errorf("%s for %s", ErrOptionNotFound, key)
		}
		if value == "" {
			continue
		}
		c.options[key] = value
	}
	return nil
}

func (c *Config) Get(key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	seachKey := strings.ToUpper(key)
	value, exists := c.options[seachKey]
	if !exists {
		return "", fmt.Errorf("%s for %s", ErrOptionNotFound, key)
	}
	return value, nil
}

func (c *Config) GetBatch(keys []string) map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	results := make(map[string]string)
	for _, key := range keys {
		seachKey := strings.ToUpper(key)
		val := c.options[seachKey]
		if val != "" {
			results[key] = c.options[seachKey]
		}
	}
	return results
}
