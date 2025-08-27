// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package configs

import (
	"crypto/hkdf"
	"crypto/sha256"
	"hash"
)

// Keys contains the encryption and signin keys.
var Keys = KeyMaterial{}

const (
	keyToken   = "api_token"
	keySession = "session"
)

// KeyMaterial contains the signing and encryption keys.
type KeyMaterial struct {
	prk        []byte // Main pseudorandom key
	tokenKey   []byte
	sessionKey []byte
}

func hkdfHashFunc() hash.Hash {
	return sha256.New()
}

// Expand call [hkdf.Expand] using the [KeyMaterial.prk] main key.
func (km KeyMaterial) Expand(name string, keyLength int) ([]byte, error) {
	if len(km.prk) != 32 {
		panic("keys not initialized")
	}
	return hkdf.Expand(hkdfHashFunc, km.prk, name, keyLength)
}

// TokenKey returns a 256-bit key used to genrate a token's MAC.
func (km KeyMaterial) TokenKey() []byte {
	return km.tokenKey
}

// SessionKey returns a 256-bit key used by the session's secure cookie.
func (km KeyMaterial) SessionKey() []byte {
	return km.sessionKey
}

func (km KeyMaterial) mustExpand(name string, keyLength int) []byte {
	k, err := km.Expand(name, keyLength)
	if err != nil {
		panic(err)
	}
	return k
}

// loadKeys prepares all the keys derivated from the configuration's
// secret key.
func loadKeys() {
	var err error

	// Initial Key Material
	Keys.prk, err = hkdf.Extract(hkdfHashFunc, []byte(Config.Main.SecretKey), nil)
	if err != nil {
		panic(err)
	}

	// Derived keys
	Keys.tokenKey = Keys.mustExpand(keyToken, 32)
	Keys.sessionKey = Keys.mustExpand(keySession, 32)
}
