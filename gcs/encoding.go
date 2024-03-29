// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package gcs

import (
	"io"
	"math"
)

type golombEncoder struct {
	inner       *bitWriter
	probability uint64
	log2p       uint8
}

func newEncoder(w io.Writer, probability uint64) *golombEncoder {
	return &golombEncoder{
		inner:       newBitWriter(w),
		probability: probability,
		log2p:       uint8(math.Ceil(math.Log2(float64(probability)))),
	}
}

// Encode golomb encodes the value and writes it to a file
func (e *golombEncoder) Encode(value uint64) (uint64, error) {
	q := value / e.probability
	r := value % e.probability
	written := uint64(0)

	if err := e.inner.WriteBits(uint8(q+1), (1<<(q+1))-2); err != nil {
		return written, err
	}
	written += q + 1

	if err := e.inner.WriteBits(e.log2p, r); err != nil {
		return written, err
	}
	written += uint64(e.log2p)
	return written, nil
}

func (e *golombEncoder) Finalize() (uint64, error) {
	return e.inner.Flush()
}
