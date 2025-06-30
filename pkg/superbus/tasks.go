// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package superbus provides an asynchronous task handler.
package superbus

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

type (
	// Operation is the event sent when we launch a task.
	Operation struct {
		Name string      `json:"name"`
		ID   interface{} `json:"id"`
	}

	// Payload is the stored content of a task.
	Payload struct {
		ID    uuid.UUID `json:"id"`
		Delay int       `json:"delay"`
		Data  []byte    `json:"data"`
	}

	// TaskHandler is the function called on a task.
	TaskHandler func(*Operation, *Payload)

	// TaskManager is the task manager.
	TaskManager struct {
		sync.Mutex

		em          EventManager
		store       Store
		handlers    map[string]TaskHandler
		queue       chan func()
		numWorkers  int
		workerGroup *sync.WaitGroup
		timerGroup  *sync.WaitGroup
		keyPrefix   string
	}

	// TaskManagerOption is a function that sets TaskManager option upon creation.
	TaskManagerOption func(*TaskManager)

	// TaskOption is a function that sets Task option upon creation.
	TaskOption func(t *Task)

	// Task is a task shortcut.
	Task struct {
		tm             *TaskManager
		name           string
		delay          int
		unmarshallData func(data []byte) interface{}
		taskHandler    func(data interface{})
	}
)

// NewTaskManager creates a new TaskManager instance.
func NewTaskManager(m EventManager, s Store, options ...TaskManagerOption) *TaskManager {
	tm := &TaskManager{
		em:          m,
		store:       s,
		handlers:    make(map[string]TaskHandler),
		queue:       make(chan func()),
		numWorkers:  1,
		workerGroup: &sync.WaitGroup{},
		timerGroup:  &sync.WaitGroup{},
		keyPrefix:   "tasks",
	}

	for _, o := range options {
		o(tm)
	}

	tm.em.On("task", tm.onTask)

	return tm
}

// WithNumWorkers set the number of worker processes that handle operations.
func WithNumWorkers(m int) TaskManagerOption {
	return func(tm *TaskManager) {
		tm.numWorkers = m
	}
}

// WithOperationPrefix sets a prefix for all operation payloads.
func WithOperationPrefix(p string) TaskManagerOption {
	return func(tm *TaskManager) {
		tm.keyPrefix = p
	}
}

// onTask is the task's event handler.
func (tm *TaskManager) onTask(e Event) {
	var op Operation
	if err := json.Unmarshal(e.Value, &op); err != nil {
		slog.Error("", slog.Any("err", err))
		return
	}
	l := slog.With(
		slog.Any("operation", op),
		slog.String("event", e.Name),
	)
	l.Debug("received event")

	// Find the task's payload
	p0, err := tm.getPayload(&op)
	if err != nil {
		l.Debug("", slog.Any("err", err))
		return
	}

	// If there's no handler, clean everything up
	f, ok := tm.handlers[op.Name]
	if !ok {
		l.Error("no handler")
		if err = tm.delPayload(&op); err != nil {
			l.Error("removing payload", slog.Any("err", err))
		}
		return
	}

	// Enqueue the task to the queue
	tm.timerGroup.Add(1)
	time.AfterFunc(time.Second*time.Duration(p0.Delay), func() {
		defer tm.timerGroup.Done()

		// Fetch the payload
		// If the payload is gone, the task was canceled
		p1, err := tm.getPayload(&op)
		if err != nil {
			l.Error("", slog.Any("err", err))
			return
		}

		// Check the original and current payload IDs. When they
		// don't match, the timer is running for a previous task and
		// we don't need it anymore.
		if p0.ID != p1.ID {
			l.Error("not matching payloads")
			return
		}

		// Push the worker to the queue.
		tm.queue <- func() {
			defer func() {
				if r := recover(); r != nil {
					l.Error("task error", slog.Any("recover", r))
				}
				// The payload can be removed when we're done.
				if err := tm.delPayload(&op); err != nil {
					l.Error("removing payload", slog.Any("err", err))
				}
			}()
			f(&op, &p1)
		}
	})
}

// getOperationKey returns the store key for an operation.
func (tm *TaskManager) getOperationKey(name string, id interface{}) string {
	return fmt.Sprintf("%s:%s:%v", tm.keyPrefix, name, id)
}

// getPayload returns the operation's payload after retrieving it from the store.
func (tm *TaskManager) getPayload(op *Operation) (payload Payload, err error) {
	data := tm.store.Get(tm.getOperationKey(op.Name, op.ID))
	if data == "" {
		err = errors.New("payload not found")
		return
	}
	err = json.Unmarshal([]byte(data), &payload)
	return
}

// delPayload removes the operation's payload from the store.
func (tm *TaskManager) delPayload(t *Operation) error {
	return tm.store.Del(tm.getOperationKey(t.Name, t.ID))
}

// Start starts the events listener and the process workers.
func (tm *TaskManager) Start() {
	go tm.em.Listen()
	for i := 0; i < tm.numWorkers; i++ {
		tm.workerGroup.Add(1)
		go func(_ int) {
			defer tm.workerGroup.Done()
			for task := range tm.queue {
				task()
			}
		}(i)
	}
}

// Stop stops the event listener and wait for running tasks to finish.
func (tm *TaskManager) Stop() {
	// Stop the event bus (can't receive any new event)
	tm.em.Stop()

	// Wait for all timers
	tm.timerGroup.Wait()

	// Stop the worker group
	close(tm.queue)
	tm.workerGroup.Wait()
}

// Launch sends a task order for later launch.
func (tm *TaskManager) Launch(name string, id interface{}, delay int, data interface{}) error {
	t := Operation{
		Name: name,
		ID:   id,
	}

	// Store the payload
	payload := Payload{
		ID:    uuid.New(),
		Delay: delay,
	}
	var err error
	if payload.Data, err = json.Marshal(data); err != nil {
		return err
	}

	p, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	err = tm.store.Set(tm.getOperationKey(name, id), string(p), time.Duration(delay)+time.Second*30)
	if err != nil {
		return err
	}

	// Send the event
	e, _ := json.Marshal(t)
	return tm.em.Push("task", e)
}

// Register registers a task handler.
func (tm *TaskManager) Register(name string, f TaskHandler) {
	tm.Lock()
	defer tm.Unlock()

	tm.handlers[name] = f
}

// NewTask creates a new Task instance.
func (tm *TaskManager) NewTask(name string, options ...TaskOption) Task {
	t := Task{
		tm:    tm,
		name:  name,
		delay: 0,
	}

	for _, o := range options {
		o(&t)
	}
	tm.Register(t.name, func(_ *Operation, p *Payload) {
		var data interface{} = p.Data
		if t.unmarshallData != nil {
			data = t.unmarshallData(p.Data)
		}
		t.taskHandler(data)
	})

	return t
}

// WithTaskHandler adds the given handler to the task.
func WithTaskHandler(f func(data interface{})) TaskOption {
	return func(t *Task) {
		t.taskHandler = f
	}
}

// WithUnmarshall registers a function that is responsible for payload decoding.
func WithUnmarshall(f func(data []byte) interface{}) TaskOption {
	return func(t *Task) {
		t.unmarshallData = f
	}
}

// WithTaskDelay sets the task's delay.
func WithTaskDelay(d int) TaskOption {
	return func(t *Task) {
		t.delay = d
	}
}

// Run launches the task.
func (t Task) Run(id interface{}, data interface{}) error {
	t.Log().Info("starting task", slog.Any("id", id))
	return t.tm.Launch(t.name, id, t.delay, data)
}

// Cancel removes the task's payload, effectively canceling it.
func (t Task) Cancel(id interface{}) error {
	t.Log().Info("canceling task", slog.Any("id", id))
	return t.tm.store.Del(t.tm.getOperationKey(t.name, id))
}

// IsRunning returns true if the task is currently running or in the queue.
func (t Task) IsRunning(id interface{}) bool {
	return t.tm.store.Get(t.tm.getOperationKey(t.name, id)) != ""
}

// Log returns a log entry for the task.
func (t Task) Log() *slog.Logger {
	return slog.With(slog.String("name", t.name))
}
