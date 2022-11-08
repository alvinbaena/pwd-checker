// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package gcs

import (
	"bufio"
	"github.com/jfcg/sorty/v2"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"pwd-checker/internal/util"
	"sync"
)

// https://github.com/rasky/gcs
// https://github.com/Freaky/gcstool
// https://giovanni.bajo.it/post/47119962313/golomb-coded-sets-smaller-than-bloom-filters

type indexPair struct {
	value  uint64
	bitPos uint64
}

type Builder struct {
	in               *os.File
	out              io.Writer
	num              uint64
	probability      uint64
	indexGranularity uint64
	values           []uint64
	stat             *status
}

// NewBuilder builder for a new GCS file database.
//
// probability is the False positive rate for queries, 1-in-p.
// indexGranularity is the entries per index point (16 bytes each).
func NewBuilder(in *os.File, out io.Writer, probability uint64, indexGranularity uint64) *Builder {
	// Estimate the amount of lines in the passwords file. It's pretty accurate, <= 1% error rate.
	// 847223402 is the exact number of lines for v8 file
	estimatedLines := estimateFileLines(in)

	return &Builder{
		in:               in,
		out:              out,
		num:              estimatedLines,
		probability:      probability,
		indexGranularity: indexGranularity,
		values:           make([]uint64, 0, estimatedLines),
	}
}

// Process creates the gcs file using the inputs in the builder
// Concurrent file read inspired by https://marcellanz.com/post/file-read-challenge/
func (b *Builder) Process(skipWait bool) error {
	// Stop the process if not enough ram to actually hold all the entries read.
	util.CheckRam(b.num, skipWait)

	s := util.Stats()
	defer s()

	b.stat = newStatus()
	log.Info().Msg("starting process. This might take a while, be patient :)")

	scanner := bufio.NewScanner(b.in)

	// Pool to store the read lines from the file, in 64kb chunks
	linesChunkLen := 64 * 1024
	linesPool := sync.Pool{New: func() interface{} {
		lines := make([]string, 0, linesChunkLen)
		return lines
	}}
	lines := linesPool.Get().([]string)[:0]

	recordsPool := sync.Pool{New: func() interface{} {
		entries := make([]uint64, 0, linesChunkLen)
		return entries
	}}

	// Mutex needed to avoid resource contention between the coroutines
	mutex := &sync.Mutex{}
	wg := sync.WaitGroup{}

	b.stat.StageWork("Read", b.num)
	// Read first line
	willScan := scanner.Scan()
	for willScan {
		lines = append(lines, scanner.Text())
		willScan = scanner.Scan()

		if len(lines) == linesChunkLen || !willScan {
			linesToProcess := lines
			wg.Add(len(linesToProcess))

			go func() {
				// Clear data
				records := recordsPool.Get().([]uint64)[:0]

				for _, line := range linesToProcess {
					hash := U64FromHex([]byte(line)[0:16])
					records = append(records, hash)
				}

				linesPool.Put(linesToProcess)
				// Avoid resource contention
				mutex.Lock()

				for _, hash := range records {
					b.stat.Incr()
					b.add(hash)
				}

				mutex.Unlock()
				recordsPool.Put(records)

				wg.Add(-len(records))
			}()

			// Clear slice
			lines = linesPool.Get().([]string)[:0]
		}
	}
	// Wait for all coroutines to finish
	wg.Wait()

	// Create the GCS file
	if err := b.finalize(); err != nil {
		return err
	}

	b.stat.Done()
	return nil
}

// Add adds a new item to the database
func (b *Builder) add(entry uint64) {
	b.values = append(b.values, entry)
}

// Finalize the construction of the database
func (b *Builder) finalize() error {
	// Adjust with the actual number of items, not the estimate
	b.num = uint64(len(b.values))
	log.Debug().Msgf("database will have %d items", b.num)

	np := b.num * b.probability

	b.stat.Stage("Normalise")
	for i, v := range b.values {
		b.values[i] = v % np
	}

	b.stat.Stage("Sort")
	sorty.SortSlice(b.values)

	b.stat.Stage("Deduplicate")
	b.values = dedup(b.values)

	indexPoints := b.num / b.indexGranularity
	index := make([]indexPair, 0, indexPoints)

	encoder := newEncoder(b.out, b.probability)
	b.stat.StageWork("Encode", b.num)

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
			index = append(index, indexPair{value: b.values[i+1], bitPos: totalBits})
		}

		b.stat.Incr()
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
	log.Debug().Msgf("end of data: %d", endOfData)
	b.stat.Stage("Write Index")
	log.Debug().Msgf("index will have %d items", len(index))

	// Write the index: pairs of u64's (value, bit index)
	for _, pair := range index {
		if _, err = b.out.Write(toFixedBytes(pair.value)); err != nil {
			return err
		}
		if _, err = b.out.Write(toFixedBytes(pair.bitPos)); err != nil {
			return err
		}
	}

	// Write our footer
	// N, P, index position in bytes, index size in entries [magic]
	// 5*8=40 bytes
	if _, err = b.out.Write(toFixedBytes(b.num)); err != nil {
		return err
	}
	if _, err = b.out.Write(toFixedBytes(b.probability)); err != nil {
		return err
	}
	if _, err = b.out.Write(toFixedBytes(endOfData)); err != nil {
		return err
	}
	if _, err = b.out.Write(toFixedBytes(uint64(len(index)))); err != nil {
		return err
	}
	if _, err = b.out.Write([]byte(gcsMagic)); err != nil {
		return err
	}

	return nil
}
