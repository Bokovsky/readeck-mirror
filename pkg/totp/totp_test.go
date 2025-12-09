// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package totp

import (
	"encoding/base32"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRFCMatrix(t *testing.T) {
	secSha1 := base32.StdEncoding.EncodeToString([]byte("12345678901234567890"))
	secSha256 := base32.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))
	secSha512 := base32.StdEncoding.EncodeToString([]byte("1234567890123456789012345678901234567890123456789012345678901234"))

	tests := []struct {
		ts   int64
		otp  string
		code Code
	}{
		{59, "94287082", Code{Algorithm: SHA1, Digits: 8, Secret: secSha1}},
		{59, "46119246", Code{Algorithm: SHA256, Digits: 8, Secret: secSha256}},
		{59, "90693936", Code{Algorithm: SHA512, Digits: 8, Secret: secSha512}},
		{1111111109, "07081804", Code{Algorithm: SHA1, Digits: 8, Secret: secSha1}},
		{1111111109, "68084774", Code{Algorithm: SHA256, Digits: 8, Secret: secSha256}},
		{1111111109, "25091201", Code{Algorithm: SHA512, Digits: 8, Secret: secSha512}},
		{1111111111, "14050471", Code{Algorithm: SHA1, Digits: 8, Secret: secSha1}},
		{1111111111, "67062674", Code{Algorithm: SHA256, Digits: 8, Secret: secSha256}},
		{1111111111, "99943326", Code{Algorithm: SHA512, Digits: 8, Secret: secSha512}},
		{1234567890, "89005924", Code{Algorithm: SHA1, Digits: 8, Secret: secSha1}},
		{1234567890, "91819424", Code{Algorithm: SHA256, Digits: 8, Secret: secSha256}},
		{1234567890, "93441116", Code{Algorithm: SHA512, Digits: 8, Secret: secSha512}},
		{2000000000, "69279037", Code{Algorithm: SHA1, Digits: 8, Secret: secSha1}},
		{2000000000, "90698825", Code{Algorithm: SHA256, Digits: 8, Secret: secSha256}},
		{2000000000, "38618901", Code{Algorithm: SHA512, Digits: 8, Secret: secSha512}},
		{20000000000, "65353130", Code{Algorithm: SHA1, Digits: 8, Secret: secSha1}},
		{20000000000, "77737706", Code{Algorithm: SHA256, Digits: 8, Secret: secSha256}},
		{20000000000, "47863826", Code{Algorithm: SHA512, Digits: 8, Secret: secSha512}},
	}

	for _, test := range tests {
		label := fmt.Sprintf("%s %s", test.code.Algorithm, test.otp)
		t.Run(label, func(t *testing.T) {
			assert := require.New(t)
			otp, err := test.code.OTP(time.Unix(test.ts, 0).UTC())
			assert.NoError(err)
			assert.Equal(test.otp, otp)

			ok, err := test.code.Verify(otp, time.Unix(test.ts, 0).UTC(), 0)
			assert.NoError(err)
			assert.True(ok)
		})
	}
}

func TestGeneric(t *testing.T) {
	sec1 := "ZYTYYE5FOAGW5ML7LRWUL4WTZLNJAMZS"
	sec2 := "PW4YAYYZVDE5RK2AOLKUATNZIKAFQLZO"

	tests := []struct {
		ts   int64
		otp  string
		code Code
	}{
		// Tests from https://github.com/creachadair/otp
		{1642868750, "349451", NewCode("aaaabbbbccccdddd")},
		{1642868800, "349712", NewCode("aaaabbbbccccdddd")},
		{1642868822, "367384", NewCode("aaaabbbbccccdddd")},
		{1642869021, "436225", NewCode("aaaabbbbccccdddd")},

		// Tests from https://github.com/susam/mintotp
		{0, "549419", NewCode(sec1)},
		{0, "009551", NewCode(sec2)},
		{10, "549419", NewCode(sec1)},
		{10, "009551", NewCode(sec2)},
		{1260, "626854", NewCode(sec1)},
		{1260, "093610", NewCode(sec2)},
		{1270, "626854", NewCode(sec1)},
		{1270, "093610", NewCode(sec2)},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			assert := require.New(t)
			otp, err := test.code.OTP(time.Unix(test.ts, 0).UTC())
			assert.NoError(err)
			assert.Equal(test.otp, otp)

			ok, err := test.code.Verify(test.otp, time.Unix(test.ts, 0).UTC(), 0)
			assert.NoError(err)
			assert.True(ok)
		})
	}
}

func TestSkew(t *testing.T) {
	tests := []struct {
		t   time.Time
		otp string
	}{
		{time.Unix(60, 0).UTC(), "336321"},
		{time.Unix(30, 0).UTC(), "663947"},
		{time.Unix(90, 0).UTC(), "128204"},
	}

	assert := require.New(t)
	code := NewCode("ETBZKJG2XXN4XY4IRV3AETV6IGICCO35")
	otp, err := code.OTP(tests[0].t)
	assert.NoError(err)
	assert.Equal(tests[0].otp, otp)

	for _, test := range tests {
		t.Run(test.otp, func(t *testing.T) {
			ok, err := code.Verify(test.otp, tests[0].t, 1)
			require.NoError(t, err)
			require.True(t, ok)
		})
	}
}

func TestGenerateSecret(t *testing.T) {
	assert := require.New(t)

	for range 100 {
		c := Generate()
		assert.NotEmpty(c.Secret)
		assert.Len(c.Secret, 32)
		_, err := base32.StdEncoding.DecodeString(c.Secret)
		assert.NoError(err)
	}
}

func TestCodeURL(t *testing.T) {
	tests := []struct {
		code     Code
		expected string
	}{
		{
			Code{
				Secret:    "ETBZKJG2XXN4XY4IRV3AETV6IGICCO35",
				Algorithm: SHA1,
				Digits:    6,
				Period:    30,
				Issuer:    "issuer",
				Account:   "user",
			},
			"otpauth://totp/issuer:user?algorithm=SHA1&digits=6&issuer=issuer&period=30&secret=ETBZKJG2XXN4XY4IRV3AETV6IGICCO35",
		},
		{
			Code{
				Secret:    "ETBZKJG2XXN4XY4IRV3AETV6IGICCO35",
				Algorithm: SHA1,
				Digits:    8,
				Period:    30,
			},
			"otpauth://totp?algorithm=SHA1&digits=8&period=30&secret=ETBZKJG2XXN4XY4IRV3AETV6IGICCO35",
		},
		{
			Code{
				Secret:    "ETBZKJG2XXN4XY4IRV3AETV6IGICCO35",
				Algorithm: SHA512,
				Digits:    8,
				Period:    60,
				Issuer:    "ACME Co",
				Account:   "user@example.org",
			},
			"otpauth://totp/ACME%20Co:user@example.org?algorithm=SHA512&digits=8&issuer=ACME%20Co&period=60&secret=ETBZKJG2XXN4XY4IRV3AETV6IGICCO35",
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			require.Equal(t, test.expected, test.code.URL().String())
		})
	}
}
