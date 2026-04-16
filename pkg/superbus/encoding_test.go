// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package superbus_test

import (
	"bytes"
	"crypto/rand"
	"encoding/gob"
	"encoding/json"
	"io"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/google/uuid"
)

type Payload struct {
	ID        int
	Rnd       string
	B         bool
	Resources []PayloadResource
}

type PayloadResource struct {
	Name string
	Data []byte
}

func makePayload(resCount, resSize int) *Payload {
	res := &Payload{
		ID:        10,
		Rnd:       uuid.Must(uuid.NewRandom()).String(),
		B:         true,
		Resources: []PayloadResource{},
	}

	for range resCount {
		r := PayloadResource{
			Name: uuid.Must(uuid.NewRandom()).String(),
			Data: make([]byte, resSize),
		}
		rand.Read(r.Data)
		res.Resources = append(res.Resources, r)
	}

	return res
}

type Encoder interface {
	Encode(v any) error
}

type Decoder interface {
	Decode(v any) error
}

func BenchmarkEncoding(b *testing.B) { //nolint:gocognit
	benchmarks := []struct {
		name     string
		resCount int
		resSize  int
	}{
		{"tiny", 0, 0},
		{"small 1x1k", 1, 1024},
		{"medium 5x1M", 5, 1 << 20},
		{"big 10x5M", 10, 5 << 20},
	}

	codecs := []struct {
		name    string
		encoder func(w io.Writer) Encoder
		decoder func(r io.Reader) Decoder
	}{
		{
			name: "json",
			encoder: func(w io.Writer) Encoder {
				return json.NewEncoder(w)
			},
			decoder: func(r io.Reader) Decoder {
				return json.NewDecoder(r)
			},
		},
		{
			name: "gob",
			encoder: func(w io.Writer) Encoder {
				return gob.NewEncoder(w)
			},
			decoder: func(r io.Reader) Decoder {
				return gob.NewDecoder(r)
			},
		},
		{
			name: "cbor",
			encoder: func(w io.Writer) Encoder {
				return cbor.NewEncoder(w)
			},
			decoder: func(r io.Reader) Decoder {
				return cbor.NewDecoder(r)
			},
		},
	}

	for _, bench := range benchmarks {
		b.Run(bench.name, func(b *testing.B) {
			payload := makePayload(bench.resCount, bench.resSize)
			b.Run("encode", func(b *testing.B) {
				for _, codec := range codecs {
					b.Run(codec.name, func(b *testing.B) {
						enc := codec.encoder(io.Discard)

						for b.Loop() {
							if err := enc.Encode(payload); err != nil {
								b.Fatal(err)
							}
						}
					})
				}
			})

			b.Run("decode", func(b *testing.B) {
				payload := makePayload(bench.resCount, bench.resSize)

				for _, codec := range codecs {
					b.Run(codec.name, func(b *testing.B) {
						buf := new(bytes.Buffer)
						if err := codec.encoder(buf).Encode(&payload); err != nil {
							b.Fatal(err)
						}
						data := buf.Bytes()

						for b.Loop() {
							var p Payload
							if err := codec.decoder(bytes.NewReader(data)).Decode(&p); err != nil {
								b.Fatal(err)
							}
						}
					})
				}
			})
		})
	}
}
