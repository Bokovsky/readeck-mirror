// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package sessions provides a cookie based session manager.
// It's heavily based on gorilla session but with a structured session
// payload that can be serialized to json.
package sessions

import (
	"hash"
	"io"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/readeck/readeck/pkg/http/securecookie"
)

// Payload contains session values.
type Payload struct {
	Seed        int            `json:"s"`
	User        int            `json:"u"`
	LastUpdate  time.Time      `json:"lu"`
	Flashes     []FlashMessage `json:"f"`
	Preferences Preferences    `json:"p"`
}

// FlashMessage is a message stored in the session.
type FlashMessage struct {
	Type    string `json:"t"`
	Message string `json:"m"`
}

// Preferences contains the user session preferences.
type Preferences struct {
	BookmarkListDisplay   string `json:"bld"`
	BookmarkSidebarHidden bool   `json:"bsh"`
}

// Session is a unique session.
type Session struct {
	handler *securecookie.Handler
	Payload *Payload
	IsNew   bool
}

// New creates or retrieves a session.
// Event if there are errors while loading and decoding the cookie,
// it always returns a [Session] instance.
func New(h *securecookie.Handler, r *http.Request) (*Session, error) {
	s := &Session{
		handler: h,
		Payload: new(Payload),
		IsNew:   true,
	}

	err := s.handler.Load(r, s.Payload)
	if err == nil {
		s.IsNew = false
	}

	return s, err
}

// Save sends the session's cookie to the HTTP response.
func (s *Session) Save(w http.ResponseWriter, r *http.Request) error {
	return s.handler.Save(w, r, s.Payload)
}

// Clear deletes the session.
func (s *Session) Clear(w http.ResponseWriter, r *http.Request) {
	s.handler.Delete(w, r)
}

// AddFlash add a new flash message to the session.
func (s *Session) AddFlash(typ, msg string) {
	s.Payload.Flashes = append(s.Payload.Flashes, FlashMessage{typ, msg})
}

// UpdateEtag implements [server.Etagger] interface.
func (s *Session) UpdateEtag(h hash.Hash) {
	io.WriteString(h, strconv.FormatInt(s.Payload.LastUpdate.UTC().UnixNano(), 10))
}

// GetLastModified implement LastModer interface.
func (s *Session) GetLastModified() []time.Time {
	return []time.Time{s.Payload.LastUpdate}
}

// Flashes retrieves the flash messages from the session
// and flushes them. The session is not saved, it's up to the code (middleware)
// that calls this function to save the session.
func (s *Session) Flashes() []FlashMessage {
	defer func() {
		s.Payload.Flashes = make([]FlashMessage, 0)
	}()
	return s.Payload.Flashes
}
