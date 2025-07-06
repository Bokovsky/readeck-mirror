// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package securecookie

import (
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var skey, _ = hex.DecodeString("3490e8d46d132052c8437c0eb02fdb1cb76546169ea9cc162f83c795621570eb")

func TestStore(t *testing.T) {
	t.Run("encode-decode", func(t *testing.T) {
		assert := require.New(t)
		s := store{
			key: Key(skey),
		}
		msg := []byte("lorem ipsum 😺")
		encoded, err := s.encode(msg)
		assert.NoError(err)

		decoded, err := s.decode(encoded)
		assert.NoError(err)
		assert.Exactly(msg, decoded)
	})

	t.Run("tampered message", func(t *testing.T) {
		assert := require.New(t)

		s := store{
			key: Key(skey),
		}
		msg := []byte("lorem ipsum 😺")
		encoded, err := s.encode(msg)
		assert.NoError(err)

		encoded[0] = ^encoded[0]

		_, err = s.decode(encoded)
		assert.ErrorContains(err, "message authentication failed")
	})

	t.Run("decode error", func(t *testing.T) {
		assert := require.New(t)

		s := store{
			key: Key(skey),
		}

		msg := []byte("abcd")
		decoded, err := s.decode(msg)
		assert.ErrorIs(err, ErrMsgTooShort)
		assert.Equal([]byte(nil), decoded)
	})
}

func TestHandler(t *testing.T) {
	h := NewHandler(Key(skey))

	t.Run("encode-decode", func(t *testing.T) {
		assert := require.New(t)
		p := []int{1, 2, 3, 4}
		encoded, err := h.Encode(p)
		assert.NoError(err)

		var d []int
		err = h.Decode(encoded, &d)
		assert.NoError(err)
		assert.Exactly(d, p)
	})

	t.Run("tampered", func(t *testing.T) {
		assert := require.New(t)
		p := []int{1, 2, 3, 4}
		encoded, err := h.Encode(p)
		assert.NoError(err)

		var d []int
		encoded[0] ^= encoded[0]
		err = h.Decode(encoded, &d)
		assert.ErrorContains(err, "message authentication failed")
		assert.Exactly([]int(nil), d)
	})

	t.Run("set cookie", func(t *testing.T) {
		assert := require.New(t)

		p := []int{1, 2, 3, 4}
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := h.Save(w, r, p)
		assert.NoError(err)
		assert.Len(w.Result().Cookies(), 1)
		cookie := w.Result().Cookies()[0]
		assert.True(cookie.HttpOnly)
		assert.Equal(http.SameSiteLaxMode, cookie.SameSite)
		assert.Equal("session", cookie.Name)
		assert.Equal("/", cookie.Path)
	})

	t.Run("load cookie", func(t *testing.T) {
		assert := require.New(t)

		h := NewHandler(Key(skey), WithName("xid"), WithPath("/test"))

		r := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		payload := []int{1, 2, 3, 4}

		err := h.Save(w, r, payload)
		assert.NoError(err)
		assert.Len(w.Result().Cookies(), 1)

		r = httptest.NewRequest("GET", "/test", nil)
		r.AddCookie(w.Result().Cookies()[0])

		var d []int
		err = h.Load(r, &d)
		assert.NoError(err)
		assert.Exactly(d, payload)
	})

	t.Run("expired", func(t *testing.T) {
		assert := require.New(t)
		tick := 0

		h := NewHandler(Key(skey), WithName("xid"), WithPath("/test"), WithMaxAge(60*121))
		h.now = func() time.Time {
			t, _ := time.Parse(time.DateTime, time.DateTime)
			t = t.Add(time.Duration(tick) * time.Hour)
			tick++
			return t
		}

		r := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		payload := []int{1, 2, 3, 4}

		err := h.Save(w, r, payload)
		assert.NoError(err)
		assert.Len(w.Result().Cookies(), 1)
		cookie := w.Result().Cookies()[0]
		assert.Equal("xid", cookie.Name)
		assert.Equal("/test", cookie.Path)

		r = httptest.NewRequest("GET", "/test", nil)
		r.AddCookie(w.Result().Cookies()[0])

		var d []int
		assert.NoError(h.Load(r, &d))

		r = httptest.NewRequest("GET", "/test", nil)
		r.AddCookie(w.Result().Cookies()[0])
		assert.ErrorIs(h.Load(r, &d), ErrExpired)
	})

	t.Run("no ttl", func(t *testing.T) {
		assert := require.New(t)
		tick := 0

		h := NewHandler(Key(skey), WithMaxAge(60*121), WithTTL(false))
		h.now = func() time.Time {
			t, _ := time.Parse(time.DateTime, time.DateTime)
			t = t.Add(time.Duration(tick) * time.Hour)
			tick++
			return t
		}

		r := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		payload := []int{1, 2, 3, 4}

		err := h.Save(w, r, payload)
		assert.NoError(err)
		assert.Len(w.Result().Cookies(), 1)

		r = httptest.NewRequest("GET", "/test", nil)
		r.AddCookie(w.Result().Cookies()[0])

		var d []int
		assert.NoError(h.Load(r, &d))

		r = httptest.NewRequest("GET", "/test", nil)
		r.AddCookie(w.Result().Cookies()[0])
		assert.NoError(err)
	})

	t.Run("marshall error", func(t *testing.T) {
		assert := require.New(t)

		p := func() {}
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := h.Save(w, r, p)
		assert.ErrorContains(err, "unsupported type")
		assert.Empty(w.Result().Cookies())
	})

	t.Run("no cookie", func(t *testing.T) {
		assert := require.New(t)
		r := httptest.NewRequest("GET", "/", nil)
		var d []int
		err := h.Load(r, &d)
		assert.ErrorIs(err, http.ErrNoCookie)
	})

	t.Run("too short", func(t *testing.T) {
		assert := require.New(t)
		r := httptest.NewRequest("GET", "/", nil)

		cookie := h.newCookie(r)
		encoded, err := h.store.encode([]byte("123"))
		assert.NoError(err)
		cookie.Value = base64.URLEncoding.EncodeToString(encoded)
		r.AddCookie(cookie)

		var payload any
		err = h.Load(r, &payload)
		assert.ErrorIs(err, ErrMsgTooShort)
	})

	t.Run("too long", func(t *testing.T) {
		assert := require.New(t)
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		p := make([]byte, 3000)
		err := h.Save(w, r, p)
		assert.ErrorIs(err, ErrMsgTooLong)
	})

	t.Run("delete", func(t *testing.T) {
		assert := require.New(t)

		h := NewHandler(Key(skey), WithName("xid"), WithPath("/test"))

		r := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		payload := []int{1, 2, 3, 4}

		err := h.Save(w, r, payload)
		assert.NoError(err)
		assert.Len(w.Result().Cookies(), 1)

		r = httptest.NewRequest("GET", "/test", nil)
		r.AddCookie(w.Result().Cookies()[0])

		var d []int
		err = h.Load(r, &d)
		assert.NoError(err)
		assert.Exactly(d, payload)

		r = httptest.NewRequest("GET", "/test", nil)
		w = httptest.NewRecorder()
		h.Delete(w, r)
		assert.Len(w.Result().Cookies(), 1)
		cookie := w.Result().Cookies()[0]
		assert.Empty(cookie.Value)
		assert.Equal(-1, cookie.MaxAge)
		assert.Equal(time.Unix(1, 0).UTC(), cookie.Expires)
	})
}
