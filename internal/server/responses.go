// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/server/urls"
)

// Message is used by the server's Message() method.
type Message struct {
	Status  int     `json:"status"`
	Message string  `json:"message"`
	Errors  []Error `json:"errors,omitempty"`
}

// Error is mainly used to return payload/querystring errors.
type Error struct {
	Location string `json:"location"`
	Error    string `json:"error"`
}

// Link contains a "Link" header information.
type Link struct {
	URL  string
	Rel  string
	Type string
}

// NewLink returns a new Link instance.
func NewLink(url string) Link {
	return Link{URL: url}
}

// WithRel adds a "rel" value to the link.
func (l Link) WithRel(rel string) Link {
	l.Rel = rel
	return l
}

// WithType adds a "type" value to the link.
func (l Link) WithType(t string) Link {
	l.Type = t
	return l
}

// Write adds the header to a ResponseWriter.
func (l Link) Write(w http.ResponseWriter) {
	h := fmt.Sprintf("<%s>", l.URL)
	if l.Rel != "" {
		h = fmt.Sprintf(`%s; rel="%s"`, h, l.Rel)
	}
	if l.Type != "" {
		h = fmt.Sprintf(`%s; type="%s"`, h, l.Type)
	}
	w.Header().Add("Link", h)
}

// Render converts any value to JSON and sends the response.
func Render(w http.ResponseWriter, r *http.Request, status int, value interface{}) {
	b := &bytes.Buffer{}
	enc := json.NewEncoder(b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		Log(r).Error("encoding error", slog.Any("err", err))
		http.Error(w, http.StatusText(500), 500)
		return
	}

	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	if status >= 100 {
		w.WriteHeader(status)
	}
	w.Write(b.Bytes())
}

// Msg sends a JSON formatted message response.
func Msg(w http.ResponseWriter, r *http.Request, message *Message) {
	Render(w, r, message.Status, message)

	// Log errors only in dev mode
	if message.Status >= 400 && configs.Config.Main.DevMode {
		Log(r).Warn(message.Message, slog.Any("message", message))
	}
}

// TextMsg sends a JSON formatted message response with a status and a message.
func TextMsg(w http.ResponseWriter, r *http.Request, status int, msg string) {
	Msg(w, r, &Message{
		Status:  status,
		Message: msg,
	})
}

// Status sends a text plain response with the given status code.
func Status(w http.ResponseWriter, _ *http.Request, status int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	fmt.Fprintln(w, http.StatusText(status))
}

// Err sends an HTTP 500 and log the given error.
func Err(w http.ResponseWriter, r *http.Request, err error) {
	Log(r).Error("server error", slog.Any("err", err))
	Status(w, r, 500)
}

// Redirect yields a 303 redirection with a location header.
// The given "ref" values are joined togegher with the server's base path
// to provide a full absolute URL.
func Redirect(w http.ResponseWriter, r *http.Request, ref ...string) {
	w.Header().Set("Location", urls.AbsoluteURL(r, ref...).String())
	w.WriteHeader(http.StatusSeeOther)
}
