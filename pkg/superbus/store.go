// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package superbus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store is a very basic key/value store.
type Store interface {
	Get(string) string
	Set(string, string, time.Duration) error
	Del(string) error
}

// RedisStore implements KvStore with redis.
type RedisStore struct {
	rdb    *redis.Client
	prefix string
}

// NewRedisStore returns a RedisStore instance. The prefix is used for each
// key operation.
func NewRedisStore(rdb *redis.Client, prefix string) *RedisStore {
	return &RedisStore{
		rdb:    rdb,
		prefix: prefix,
	}
}

// key returns a keys with the given namespace prefix.
func (s *RedisStore) key(key string) string {
	return fmt.Sprintf("%s:%s", s.prefix, key)
}

// Get returns a value for the given key. Returns an empty string when the
// value does not exist.
func (s *RedisStore) Get(key string) string {
	res, err := s.rdb.Get(context.Background(), s.key(key)).Result()
	if err == redis.Nil {
		return ""
	}

	return res
}

// Set insert or replace the value for the given key.
func (s *RedisStore) Set(key, value string, expiration time.Duration) error {
	_, err := s.rdb.Set(context.Background(), s.key(key), value, expiration).Result()
	return err
}

// Del removes the given key.
func (s *RedisStore) Del(key string) error {
	_, err := s.rdb.Del(context.Background(), s.key(key)).Result()
	return err
}

// MemStore is a KvStore implementation using a simple in memory map.
type MemStore struct {
	sync.RWMutex
	data map[string]string
}

// NewMemStore returns a MemStore instance.
func NewMemStore() *MemStore {
	return &MemStore{
		data: make(map[string]string),
	}
}

// Get returns a value for the given key. Returns an empty string when the
// value does not exist.
func (s *MemStore) Get(key string) string {
	s.RLock()
	defer s.RUnlock()
	return s.data[key]
}

// Set insert or replace the value for the given key.
func (s *MemStore) Set(key, value string, expiration time.Duration) error {
	s.Lock()
	defer s.Unlock()
	s.data[key] = value

	if expiration > 0 {
		time.AfterFunc(expiration, func() {
			s.Lock()
			defer s.Unlock()
			delete(s.data, key)
		})
	}

	return nil
}

// Del removes the given key.
func (s *MemStore) Del(key string) error {
	s.Lock()
	defer s.Unlock()
	delete(s.data, key)
	return nil
}

// Clear deletes everything in the memory store.
func (s *MemStore) Clear() {
	s.Lock()
	defer s.Unlock()
	s.data = make(map[string]string)
}
