// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package bus provides Readeck's message bus and task executor.
package bus

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/pkg/superbus"
)

var (
	rdc          *redis.Client
	protocol     string
	eventManager superbus.EventManager
	store        superbus.Store
	taskManager  *superbus.TaskManager
	readyFuncs   []func()
)

// OnReady lets you add functions that are called when the bus is loaded.
func OnReady(f func()) {
	if readyFuncs == nil {
		readyFuncs = []func(){}
	}

	readyFuncs = append(readyFuncs, f)
}

// Load loads the configuration and prepares the worker queue.
func Load() error {
	dsn, err := url.Parse(configs.Config.Worker.DSN)
	if err != nil {
		return err
	}

	switch dsn.Scheme {
	case "memory":
		eventManager = superbus.NewEagerEventManager()
		store = superbus.NewMemStore()
	case "redis":
		startRedis(dsn)
		eventManager = superbus.NewRedisEventManager(rdc)
		store = superbus.NewRedisStore(rdc, "readeck")
	default:
		return fmt.Errorf("cannot load worker protocol %s", dsn.Scheme)
	}
	protocol = dsn.Scheme

	initTaskManager()
	return nil
}

// LoadCustom loads a custom EventManager and Store. Used for tests.
func LoadCustom(p string, em superbus.EventManager, s superbus.Store) {
	protocol = p
	eventManager = em
	store = s
	initTaskManager()
}

func initTaskManager() {
	taskManager = superbus.NewTaskManager(
		eventManager, store,
		superbus.WithOperationPrefix("tasks"),
		superbus.WithNumWorkers(configs.Config.Extractor.NumWorkers),
	)

	for _, f := range readyFuncs {
		f()
	}
}

// Protocol returns the superbus protocol in use.
func Protocol() string {
	return protocol
}

// Tasks returns the default task manager.
func Tasks() *superbus.TaskManager {
	return taskManager
}

// Events returns the default event manager.
func Events() superbus.EventManager {
	return eventManager
}

// Store returns the default store.
func Store() superbus.Store {
	return store
}

func startRedis(dsn *url.URL) {
	if rdc != nil {
		return
	}

	// Default port
	if dsn.Port() == "" {
		dsn.Host = fmt.Sprintf("%s:%d", dsn.Host, 6379)
	}

	// Password
	password, _ := dsn.User.Password()

	// DB number
	p := strings.Split(strings.TrimPrefix(dsn.Path, "/"), string(os.PathSeparator))
	db := 0
	if len(p) > 0 {
		var err error
		db, err = strconv.Atoi(p[0])
		if err != nil {
			panic(err)
		}
	}

	rdc = redis.NewClient(&redis.Options{
		Addr:     dsn.Host,
		Username: dsn.User.Username(),
		Password: password,
		DB:       db,
	})
}
