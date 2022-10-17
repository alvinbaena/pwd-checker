package gcs

import (
	"bufio"
	"encoding/binary"
	"errors"
	"github.com/jfcg/sorty/v2"
	"github.com/rs/zerolog/log"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"io"
	"math"
	"os"
	"pwd-checker/internal/util"
	"sync"
	"time"
)

// https://github.com/rasky/gcs
// https://github.com/Freaky/gcstool
// https://giovanni.bajo.it/post/47119962313/golomb-coded-sets-smaller-than-bloom-filters
const gcsMagic = "[GCS:v0]"

type indexPair struct {
	value  uint64
	bitPos uint64
}

type builder struct {
	in               *os.File
	out              *os.File
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
func NewBuilder(in *os.File, out *os.File, probability uint64, indexGranularity uint64) *builder {
	// Estimate the amount of lines in the passwords file. It's pretty accurate, <= 1% error rate
	// 847223402 is the exact number of lines for v8 file
	estimatedLines := estimateFileLines(in)

	return &builder{
		in:               in,
		out:              out,
		num:              estimatedLines,
		probability:      probability,
		indexGranularity: indexGranularity,
		values:           make([]uint64, 0, estimatedLines),
	}
}

// Process creates the gcs file using the inputs in the builder
// Inspired by https://marcellanz.com/post/file-read-challenge/
func (b *builder) Process() error {
	util.CheckRam(b.num)

	s := util.Stats()
	defer s()

	time.Sleep(10 * time.Second)

	b.stat = newStatus()
	log.Info().Msg("Starting process. This might take a while, be patient :)")

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

	b.stat.StageWork("Hashing", b.num)
	// Read first line
	scanner.Scan()
	for {
		lines = append(lines, scanner.Text())
		willScan := scanner.Scan()

		if len(lines) == linesChunkLen || !willScan {
			linesToProcess := lines
			wg.Add(len(linesToProcess))

			go func() {
				// Clear data
				records := recordsPool.Get().([]uint64)[:0]

				for _, line := range linesToProcess {
					if len(line) < 16 {
						log.Trace().Msgf("Skipping line %s", line)
					} else {
						hash := U64FromHex([]byte(line)[0:16])
						records = append(records, hash)
					}
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

		if !willScan {
			break
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
func (b *builder) add(entry uint64) {
	b.values = append(b.values, entry)
}

// Finalize the construction of the database
func (b *builder) finalize() error {
	// Adjust with the actual number of items, not the estimate
	b.num = uint64(len(b.values))
	log.Debug().Msgf("Database will have %d items", b.num)

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
	log.Debug().Msgf("End of data: %d", endOfData)
	b.stat.Stage("Write Index")
	log.Debug().Msgf("Index will have %d items", len(index))

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

type Reader struct {
	fileName    string
	num         uint64
	probability uint64
	endOfData   uint64
	indexLen    uint64
	index       []indexPair
	log2p       uint8
}

func NewReader(fileName string) *Reader {
	return &Reader{
		fileName:    fileName,
		num:         0,
		probability: 0,
		endOfData:   0,
		indexLen:    0,
		index:       make([]indexPair, 0, 0),
		log2p:       0,
	}
}

func (r *Reader) Initialize() error {
	file, err := os.OpenFile(r.fileName, os.O_RDONLY, 444)
	if err != nil {
		return err
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatal().Err(err).Msg("Error closing GCS file file")
		}
	}(file)

	if _, err = file.Seek(-40, io.SeekEnd); err != nil {
		return err
	}

	buf := make([]byte, 8)
	if _, err = file.Read(buf); err != nil {
		return err
	}
	r.num = binary.BigEndian.Uint64(buf)
	log.Debug().Msgf("Number of items: %d", r.num)

	buf = make([]byte, 8)
	if _, err = file.Read(buf); err != nil {
		return err
	}
	r.probability = binary.BigEndian.Uint64(buf)
	log.Debug().Msgf("Probability: %d", r.probability)

	r.log2p = uint8(math.Ceil(math.Log2(float64(r.probability))))
	log.Debug().Msgf("Log2: %d", r.log2p)

	buf = make([]byte, 8)
	if _, err = file.Read(buf); err != nil {
		return err
	}
	r.endOfData = binary.BigEndian.Uint64(buf)
	log.Debug().Msgf("End of Data: %d", r.endOfData)

	buf = make([]byte, 8)
	if _, err = file.Read(buf); err != nil {
		return err
	}
	r.indexLen = binary.BigEndian.Uint64(buf)
	log.Debug().Msgf("Index Length: %d", r.indexLen)

	buf = make([]byte, 8)
	if _, err = file.Read(buf); err != nil {
		return err
	}

	if string(buf) != gcsMagic {
		return errors.New("not a GCS File")
	}

	if _, err = file.Seek(int64(r.endOfData), io.SeekStart); err != nil {
		return err
	}

	// slurp in the index.
	r.index = make([]indexPair, 0, 1+r.indexLen)
	r.index = append(r.index, indexPair{0, 0})

	log.Info().Msg("Initializing database")
	for i := uint64(0); i < r.indexLen; i++ {
		buf = make([]byte, 8)
		if _, err = file.Read(buf); err != nil {
			return err
		}
		val := binary.BigEndian.Uint64(buf)

		buf = make([]byte, 8)
		if _, err = file.Read(buf); err != nil {
			return err
		}
		bitPos := binary.BigEndian.Uint64(buf)

		r.index = append(r.index, indexPair{value: val, bitPos: bitPos})
	}

	p := message.NewPrinter(language.English)
	log.Info().Msgf("Ready for queries on %s items with a 1 in %s false-positive rate.", p.Sprintf("%d", r.num), p.Sprintf("%d", r.probability))
	return nil
}

func (r *Reader) Exists(target uint64) (bool, error) {
	s := util.Stats()
	defer s()

	file, err := os.OpenFile(r.fileName, os.O_RDONLY, 444)
	if err != nil {
		return false, err
	}

	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			log.Fatal().Err(err).Msg("Error closing GCS file file")
		}
	}(file)

	h := target % (r.num * r.probability)
	// Try to find exact match
	exact, closest := binarySearch(r.index, h)
	if exact > 0 {
		return true, nil
	}

	// We have to get the low value, not the latest if the item is not found.
	// This is equivalent to rust's saturating_sub(1) function on binary_search_by_key
	lastEntry := r.index[closest-1]

	// To avoid problems on concurrent file access
	//r.mutex.Lock()
	//defer r.mutex.Unlock()
	reader := newBitReader(file)
	if _, err = reader.Seek(int64(lastEntry.bitPos), io.SeekStart); err != nil {
		return false, err
	}

	last := lastEntry.value
	for last < h {
		diff := uint64(0)

		for {
			re, err := reader.ReadBit()
			if err != nil {
				return false, err
			}

			if re == 1 {
				diff += r.probability
			} else {
				break
			}
		}

		re, err := reader.ReadBits(r.log2p)
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
