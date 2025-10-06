// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package superbus

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Event is an event sent to the wire. It contains a name and a value that can be unmarshalled later.
type Event struct {
	Name  string `json:"name"`
	Value []byte `json:"value"`
}

// EventHandler is a function that deals with an event.
type EventHandler func(Event)

// EventManager describes the methods implemented by an event manager.
type EventManager interface {
	// Listen starts listening to events
	Listen()
	// Stop stops the event listener
	Stop()
	// Push sends an event
	Push(name string, value []byte) error
	// On register a callaback for a specific event
	On(name string, f EventHandler)
}

// EagerEventManager is a simple event manager using channels for event management.
type EagerEventManager struct {
	ch       chan Event
	wg       *sync.WaitGroup
	handlers map[string]EventHandler
}

// NewEagerEventManager create an EagerEventManager instance.
func NewEagerEventManager() *EagerEventManager {
	return &EagerEventManager{
		ch:       make(chan Event),
		wg:       &sync.WaitGroup{},
		handlers: make(map[string]EventHandler),
	}
}

// Listen listens for new events.
func (m *EagerEventManager) Listen() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for e := range m.ch {
			if f, ok := m.handlers[e.Name]; ok {
				f(e)
			}
		}
	}()
}

// Stop stops the event listener.
func (m *EagerEventManager) Stop() {
	close(m.ch)
	m.wg.Wait()
}

// Push sents an event to the event channel.
func (m *EagerEventManager) Push(name string, value []byte) error {
	m.ch <- Event{Name: name, Value: value}
	return nil
}

// On registers a event handler for a given event.
func (m *EagerEventManager) On(name string, f EventHandler) {
	m.handlers[name] = f
}

// RedisEventManager is an event manager using redis as a message handler.
type RedisEventManager struct {
	rdc      *redis.Client
	wg       *sync.WaitGroup
	stop     chan struct{}
	handlers map[string]EventHandler
	listName string
}

// NewRedisEventManager creates a RedisEventManager instance.
func NewRedisEventManager(rdc *redis.Client) *RedisEventManager {
	return &RedisEventManager{
		rdc:      rdc,
		wg:       &sync.WaitGroup{},
		stop:     make(chan struct{}),
		handlers: make(map[string]EventHandler),
		listName: "events",
	}
}

// Listen listens for new events.
func (m *RedisEventManager) Listen() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			select {
			case <-m.stop:
				return
			default:
				result, err := m.rdc.BLPop(
					context.Background(),
					time.Second*1,
					m.listName,
				).Result()
				if err != nil {
					continue
				}

				evt := result[1]
				e := Event{}
				if err := json.Unmarshal([]byte(evt), &e); err != nil {
					slog.Error("loading event", slog.Any("err", err))
					continue
				}

				if f, ok := m.handlers[e.Name]; ok {
					f(e)
				}
			}
		}
	}()
}

// Stop stops the event listener.
func (m *RedisEventManager) Stop() {
	m.stop <- struct{}{}
	m.wg.Wait()
}

// Push sends an event to the event channel.
func (m *RedisEventManager) Push(name string, value []byte) error {
	e := Event{Name: name, Value: value}
	b := new(bytes.Buffer)
	enc := json.NewEncoder(b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e); err != nil {
		return err
	}

	_, err := m.rdc.RPush(context.Background(), m.listName, b.String()).Result()
	return err
}

// On registers a event handler for a given event.
func (m *RedisEventManager) On(name string, f EventHandler) {
	m.handlers[name] = f
}
