// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package configs

import (
	"crypto/subtle"
	"errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/blake2b"

	"codeberg.org/readeck/readeck/pkg/base58"
)

const signMACSize = 19 // Token hash size (152bit)

var (
	// ErrInvalidSize is returned when an encoded string is not 128-bit long.
	ErrInvalidSize = errors.New("invalid encoded string size")
	// ErrInvalidData is returned when input data does not match its signature.
	ErrInvalidData = errors.New("invalid data")
)

// SigningKey is a key that can be used to sign and verify
// a base58 payload.
type SigningKey []byte

// Encode returns a base58 encoded token and its mac.
// The whole token is 280-bit long.
func (s SigningKey) Encode(raw string) (string, error) {
	id, err := base58.DecodeUUID(raw)
	if err != nil {
		return "", err
	}

	h, err := blake2b.New(signMACSize, s)
	if err != nil {
		return "", err
	}
	h.Write(id[:])

	res := make([]byte, len(id)+signMACSize)
	copy(res, id[:])
	copy(res[len(id):], h.Sum(nil))

	return base58.EncodeToString(res), nil
}

// Decode returns a token ID, using the signed, base58 encoded
// token. It checks its length and mac.
func (s SigningKey) Decode(encoded string) (string, error) {
	msg, err := base58.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	if len(msg) != 16+signMACSize {
		return "", ErrInvalidSize
	}

	id, mac := msg[:16], msg[16:]

	h, err := blake2b.New(signMACSize, s)
	if err != nil {
		return "", err
	}
	h.Write(id)

	if subtle.ConstantTimeCompare(mac, h.Sum(nil)) != 1 {
		return "", ErrInvalidData
	}

	return base58.EncodeUUID(uuid.UUID(id)), nil
}
