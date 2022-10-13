package gcs

import (
	"math"
	"os"
)

type GolombEncoder struct {
	inner       *BitWriter
	probability uint64
	log2p       uint8
}

func NewEncoder(w *os.File, probability uint64) *GolombEncoder {
	return &GolombEncoder{
		inner:       NewBitWriter(w),
		probability: probability,
		log2p:       uint8(math.Ceil(math.Log2(float64(probability)))),
	}
}

func (e *GolombEncoder) Encode(value uint64) (uint64, error) {
	q := value / e.probability
	r := value % e.probability
	written := uint64(0)

	_, err := e.inner.WriteBits(uint8(q+1), (1<<(q+1))-2)
	if err != nil {
		return written, err
	}
	written += q + 1

	_, err = e.inner.WriteBits(e.log2p, r)
	if err != nil {
		return written, err
	}
	written += uint64(e.log2p)
	return written, nil
}

func (e *GolombEncoder) Finalize() (uint64, error) {
	return e.inner.Flush()
}
