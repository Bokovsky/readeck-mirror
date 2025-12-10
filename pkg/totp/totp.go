// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package totp provides the building block the generate
// and check a TOTP key.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"maps"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

var (
	errInvalidBase32     = errors.New("invalid secret")
	errInvalidCodeLength = errors.New("code length unexpected")
)

// Algorithm is a TOTP algorithm.
type Algorithm uint8

const (
	// SHA1 is the default, google authenticator compatible, algorithm.
	SHA1 Algorithm = iota
	// SHA256 is the sha256 hmac algorithm.
	SHA256
	// SHA512 is the sha256 hmac algorithm.
	SHA512
)

func (a Algorithm) String() string {
	switch a {
	case SHA1:
		return "SHA1"
	case SHA256:
		return "SHA256"
	case SHA512:
		return "SHA512"
	}

	panic("invalid algorithm")
}

// New returns a [hash.Hash] based on the algorithm value.
func (a Algorithm) New() hash.Hash {
	switch a {
	case SHA1:
		return sha1.New() //nolint:gosec
	case SHA256:
		return sha256.New()
	case SHA512:
		return sha512.New()
	}

	panic("invalid algorithm")
}

// Code contains all the TOTP code information.
// It can verify an OTP code and generate an URL for QR codes.
type Code struct {
	Algorithm Algorithm `json:"a"`
	Digits    uint8     `json:"d"`
	Period    uint8     `json:"p"`
	Secret    string    `json:"s"`
	Issuer    string    `json:"i"`
	Account   string    `json:"u"`
}

// NewCode returns a [Code] that's compatible with Google Authenticator.
// (SHA1, 6 digit, 30s).
func NewCode(secret string) Code {
	return Code{
		Algorithm: SHA1,
		Digits:    6,
		Period:    30,
		Secret:    secret,
	}
}

// Generate returns a new [Code], Google Authenticator compatible,
// with a 32 character random secret.
func Generate() Code {
	return NewCode(GenerateSecret())
}

// URL returns a [url.URL] of the code as defined in
// https://github.com/google/google-authenticator/wiki/Key-Uri-Format
func (c Code) URL() *url.URL {
	v := url.Values{
		"secret":    {c.Secret},
		"algorithm": {c.Algorithm.String()},
		"digits":    {strconv.FormatUint(uint64(c.Digits), 10)},
		"period":    {strconv.FormatUint(uint64(c.Period), 10)},
	}

	if c.Issuer != "" {
		v.Set("issuer", c.Issuer)
	}

	u := &url.URL{
		Scheme:   "otpauth",
		Host:     "totp",
		Path:     c.Issuer,
		RawQuery: encodeQuery(v),
	}
	if c.Account != "" {
		u.Path += ":" + c.Account
	}

	return u
}

// OTP returns a user code valid for the given time.
func (c Code) OTP(t time.Time) (string, error) {
	secret := strings.TrimSpace(c.Secret)
	secret = strings.ToUpper(secret)
	if n := len(secret) % 8; n != 0 {
		secret += strings.Repeat("=", 8-n)
	}

	secretBytes, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errInvalidBase32, err)
	}

	counter := uint64(math.Floor(float64(t.Unix()) / float64(30)))

	buf := make([]byte, 8)
	mac := hmac.New(c.Algorithm.New, secretBytes)
	binary.BigEndian.PutUint64(buf, counter)
	mac.Write(buf)
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0xf
	value := int64(((int(sum[offset]) & 0x7f) << 24) |
		((int(sum[offset+1] & 0xff)) << 16) |
		((int(sum[offset+2] & 0xff)) << 8) |
		(int(sum[offset+3]) & 0xff))

	mod := int32(value % int64(math.Pow10(int(c.Digits))))
	return fmt.Sprintf("%0*d", c.Digits, mod), nil
}

// Verify returns true when a given user code is valid in the given time.
// The skew parameter can stretch (left and right) the period of validity.
func (c Code) Verify(otp string, t time.Time, skew uint8) (bool, error) {
	otp = strings.TrimSpace(otp)
	if len(otp) != int(c.Digits) {
		return false, errInvalidCodeLength
	}

	times := []time.Time{t}
	for range skew {
		times = append(times, t.Add(-time.Duration(c.Period)*time.Second))
		times = append(times, t.Add(time.Duration(c.Period)*time.Second))
	}

	for _, x := range times {
		code, err := c.OTP(x)
		if err != nil {
			return false, err
		}
		if subtle.ConstantTimeCompare([]byte(code), []byte(otp)) == 1 {
			return true, nil
		}
	}
	return false, nil
}

// GenerateSecret returns a 20-bytes, hex encoded, random value (32 characters).
func GenerateSecret() string {
	b := make([]byte, 20)
	rand.Read(b)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}

func encodeQuery(v url.Values) string {
	if v == nil {
		return ""
	}
	var buf strings.Builder
	for _, k := range slices.Sorted(maps.Keys(v)) {
		vs := v[k]
		keyEspaced := url.PathEscape(k)
		for _, v := range vs {
			if buf.Len() > 0 {
				buf.WriteByte('&')
			}
			buf.WriteString(keyEspaced)
			buf.WriteByte('=')
			buf.WriteString(url.PathEscape(v))
		}
	}
	return buf.String()
}
