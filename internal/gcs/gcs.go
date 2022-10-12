package gcs

import (
	"encoding/binary"
	"errors"
	"github.com/jfcg/sorty/v2"
	"github.com/rs/zerolog/log"
	"io"
	"math"
	"os"
)

// https://github.com/rasky/gcs
// https://github.com/Freaky/gcstool
// https://giovanni.bajo.it/post/47119962313/golomb-coded-sets-smaller-than-bloom-filters
const gcsMagic = "[GCS:v0]"

type indexPair struct {
	value  uint64
	bitPos uint64
}

type Builder struct {
	inner            *os.File
	num              uint64
	probability      uint64
	indexGranularity uint64
	values           []uint64
}

// NewBuilder builder for a new GCS file database.
//
// num is the number of items to insert into the database.
// probability is the False positive rate for queries, 1-in-p.
// indexGranularity is the entries per index point (16 bytes each).
func NewBuilder(w *os.File, num uint64, probability uint64, indexGranularity uint64) *Builder {
	return &Builder{
		inner:            w,
		num:              num,
		probability:      probability,
		indexGranularity: indexGranularity,
		values:           make([]uint64, 0, num),
	}
}

// Add adds a new item to the database
func (b *Builder) Add(entry uint64) {
	log.Trace().Msgf("Adding entry to index: %v. Size: %d", entry, len(b.values))
	b.values = append(b.values, entry)
}

// Finalize the construction of the database
func (b *Builder) Finalize(stat *Status) error {
	// Adjust with the actual number of items, not the line estimate
	b.num = uint64(len(b.values))

	log.Debug().Msgf("Index has %d items", b.num)
	np := b.num * b.probability

	stat.Stage("Normalise")
	for i, v := range b.values {
		b.values[i] = v % np
	}

	stat.Stage("Sort")
	sorty.SortSlice(b.values)

	stat.Stage("Deduplicate")
	b.values = dedup(b.values)

	indexPoints := b.num / b.indexGranularity

	index := make([]indexPair, 0, indexPoints)
	encoder := NewEncoder(b.inner, b.probability)
	stat.Stage("Encode")

	// Add a 0 at the start
	index = append(index, indexPair{0, 0})

	totalBits := uint64(0)
	for i := uint64(0); i < uint64(len(b.values)-1); i++ {
		d, err := encoder.Encode(b.values[i+1] - b.values[i])
		if err != nil {
			return err
		}
		totalBits += d

		if b.indexGranularity > 0 && i > 0 && i%b.indexGranularity == 0 {
			// Check if this is actually a correct translation of this
			// https://github.com/Freaky/gcstool/blob/6c09458986e9494cac0fee595d1f3aee6ea73636/src/gcs.rs#L116
			index = append(index, indexPair{value: b.values[i+1], bitPos: totalBits})
		}

		stat.Incr()
	}

	// encode a delimiting zero
	d, err := encoder.Encode(0)
	if err != nil {
		return err
	}
	totalBits += d

	wr, err := encoder.Finalize()
	if err != nil {
		return err
	}

	endOfData := (totalBits + wr) / 8
	stat.Stage("Index")

	// Write the index: pairs of u64's (value, bit index)
	for _, pair := range index {
		if _, err = (*b.inner).Write(toFixedBytes(pair.value)); err != nil {
			return err
		}
		if _, err = (*b.inner).Write(toFixedBytes(pair.bitPos)); err != nil {
			return err
		}
	}

	// Write our footer
	// N, P, index position in bytes, index size in entries [magic]
	// 5*8=40 bytes
	if _, err = (*b.inner).Write(toFixedBytes(b.num)); err != nil {
		return err
	}
	if _, err = (*b.inner).Write(toFixedBytes(b.probability)); err != nil {
		return err
	}
	if _, err = (*b.inner).Write(toFixedBytes(endOfData)); err != nil {
		return err
	}
	if _, err = (*b.inner).Write(toFixedBytes(uint64(len(index)))); err != nil {
		return err
	}
	if _, err = b.inner.Write([]byte(gcsMagic)); err != nil {
		return err
	}

	return nil
}

type Reader struct {
	inner       *BitReader
	num         uint64
	probability uint64
	endOfData   uint64
	indexLen    uint64
	index       []indexPair
	log2p       uint8
}

func NewReader(file *os.File) *Reader {
	return &Reader{
		inner:       NewBitReader(file),
		num:         0,
		probability: 0,
		endOfData:   0,
		indexLen:    0,
		index:       make([]indexPair, 0, 0),
		log2p:       0,
	}
}

func (r *Reader) Initialize() error {
	if _, err := r.inner.IntoInner().Seek(-40, io.SeekEnd); err != nil {
		return err
	}

	buf := make([]byte, 8)
	if _, err := r.inner.IntoInner().Read(buf); err != nil {
		return err
	}
	r.num = binary.BigEndian.Uint64(buf)
	log.Debug().Msgf("Number of items: %d", r.num)

	buf = make([]byte, 8)
	if _, err := r.inner.IntoInner().Read(buf); err != nil {
		return err
	}
	r.probability = binary.BigEndian.Uint64(buf)
	log.Debug().Msgf("Probability: %d", r.probability)

	r.log2p = uint8(math.Ceil(math.Log2(float64(r.probability))))
	log.Debug().Msgf("Log2: %d", r.log2p)

	buf = make([]byte, 8)
	if _, err := r.inner.IntoInner().Read(buf); err != nil {
		return err
	}
	r.endOfData = binary.BigEndian.Uint64(buf)
	log.Debug().Msgf("End of Data: %d", r.endOfData)

	buf = make([]byte, 8)
	if _, err := r.inner.IntoInner().Read(buf); err != nil {
		return err
	}
	r.indexLen = binary.BigEndian.Uint64(buf)
	log.Debug().Msgf("Index Length: %d", r.indexLen)

	buf = make([]byte, 8)
	if _, err := r.inner.IntoInner().Read(buf); err != nil {
		return err
	}

	if string(buf) != gcsMagic {
		return errors.New("not a GCS File")
	}

	if _, err := r.inner.IntoInner().Seek(int64(r.endOfData), io.SeekStart); err != nil {
		return err
	}

	// slurp in the index.
	r.index = make([]indexPair, 0, 1+r.indexLen)
	r.index = append(r.index, indexPair{0, 0})

	log.Info().Msg("Initializing database")
	for i := uint64(0); i < r.indexLen; i++ {
		buf = make([]byte, 8)
		if _, err := r.inner.IntoInner().Read(buf); err != nil {
			return err
		}
		val := binary.BigEndian.Uint64(buf)

		buf = make([]byte, 8)
		if _, err := r.inner.IntoInner().Read(buf); err != nil {
			return err
		}
		bitPos := binary.BigEndian.Uint64(buf)

		r.index = append(r.index, indexPair{value: val, bitPos: bitPos})
	}

	log.Info().Msgf("Ready for queries on %d items with a 1 in %d false-positive rate.", r.num, r.probability)
	return nil
}

func (r *Reader) Exists(target uint64) (bool, error) {
	h := target % (r.num * r.probability)
	// Try to find exact match
	exact, closest := binarySearch(r.index, h)
	if exact > 0 {
		return true, nil
	}

	// We have to get the low value, not the latest if the item is not found.
	// This is equivalent to rust's saturating_sub(1) function on binary_search_by_key
	lastEntry := r.index[closest-1]
	if _, err := r.inner.Seek(int64(lastEntry.bitPos), io.SeekStart); err != nil {
		return false, err
	}

	last := lastEntry.value
	for last < h {
		diff := uint64(0)

		for {
			re, err := r.inner.ReadBit()
			if err != nil {
				return false, err
			}

			if re == 1 {
				diff += r.probability
			} else {
				break
			}
		}

		re, err := r.inner.ReadBits(r.log2p)
		if err != nil {
			return false, err
		}

		diff += re
		last += diff

		// End of file
		if diff == 0 {
			break
		}
	}

	return last == h, nil
}

func toFixedBytes(content uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, content)
	return buf
}

func dedup(slice []uint64) []uint64 {
	if len(slice) < 2 {
		return slice
	}

	var e = 1
	for i := 1; i < len(slice); i++ {
		if slice[i] == slice[i-1] {
			continue
		}
		slice[e] = slice[i]
		e++
	}

	return slice[:e]
}

func binarySearch(index []indexPair, value uint64) (int, int) {
	r := -1 // not found
	start := 0
	end := len(index) - 1
	last := start
	for start <= end {
		last = start + (end-start)/2
		val := index[last]

		if val.value == value {
			r = last // found
			break
		} else if value > val.value {
			start = last + 1
		} else {
			end = last - 1
		}
	}

	return r, last
}
