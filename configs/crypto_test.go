// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package configs

import (
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"testing"

	"codeberg.org/readeck/readeck/pkg/base58"
	"github.com/stretchr/testify/require"
)

func TestSigner(t *testing.T) {
	key1, err := hkdf.Key(sha256.New, []byte("test"), nil, "sign_key1", 32)
	require.NoError(t, err)

	key2, err := hkdf.Key(sha256.New, []byte("test"), nil, "sign_key2", 32)
	require.NoError(t, err)

	s1 := SigningKey(key1)
	s2 := SigningKey(key2)

	t.Run("ok", func(t *testing.T) {
		assert := require.New(t)

		token := base58.NewUUID()
		encoded, err := s1.Encode(token)
		assert.NoError(err)

		decoded, err := s1.Decode(encoded)
		assert.NoError(err)
		assert.Equal(token, decoded)
	})

	t.Run("tampered", func(t *testing.T) {
		assert := require.New(t)

		token := base58.NewUUID()
		encoded, err := s1.Encode(token)
		assert.NoError(err)

		decoded, err := s2.Decode(encoded)
		assert.ErrorIs(err, ErrInvalidData)
		assert.Empty(decoded)
	})

	t.Run("length", func(t *testing.T) {
		assert := require.New(t)

		data := make([]byte, 12)
		rand.Read(data)

		decoded, err := s1.Decode(base58.EncodeToString(data))
		assert.ErrorIs(err, ErrInvalidSize)
		assert.Empty(decoded)
	})
}

func TestEncoder(t *testing.T) {
	key1, err := hkdf.Key(sha256.New, []byte("test"), nil, "encode_key1", 32)
	require.NoError(t, err)

	key2, err := hkdf.Key(sha256.New, []byte("test"), nil, "encode_key2", 32)
	require.NoError(t, err)

	e1 := EncodingKey(key1)
	e2 := EncodingKey(key2)

	type Foo struct {
		A string
		B []byte
		C int
	}

	t.Run("ok", func(t *testing.T) {
		assert := require.New(t)

		data := Foo{A: "abc", B: []byte("xyz"), C: 12}
		encoded, err := e1.EncodeJSON(data)
		assert.NoError(err)

		decoded := Foo{}
		err = e1.DecodeJSON(encoded, &decoded)
		assert.NoError(err)
		assert.Equal(data, decoded)
	})

	t.Run("wrong key", func(t *testing.T) {
		assert := require.New(t)

		data := Foo{A: "abc", B: []byte("xyz"), C: 12}
		encoded, err := e1.EncodeJSON(data)
		assert.NoError(err)

		decoded := Foo{}
		err = e2.DecodeJSON(encoded, &decoded)
		assert.ErrorIs(err, ErrInvalidData)
		assert.ErrorContains(err, "message authentication failed")
		assert.Equal(Foo{}, decoded)
	})

	t.Run("length", func(t *testing.T) {
		assert := require.New(t)

		data := make([]byte, 12)
		rand.Read(data)
		decoded := Foo{}

		err := e1.DecodeJSON(data, &decoded)
		assert.ErrorIs(err, ErrInvalidSize)
	})
}
