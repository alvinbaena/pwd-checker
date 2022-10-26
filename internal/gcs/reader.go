package gcs

import (
	"encoding/binary"
	"errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"io"
	"math"
	"os"
	"pwd-checker/internal/util"
)

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
	r := &Reader{
		fileName:    fileName,
		num:         0,
		probability: 0,
		endOfData:   0,
		indexLen:    0,
		index:       make([]indexPair, 0, 0),
		log2p:       0,
	}

	return r
}

// Initialize only loads the database index into memory. This does not load the whole file in RAM.
func (r *Reader) Initialize() error {
	// Only open the file for initialization.
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

	// Reads the footer that the file should have. 40 bytes.
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

	// Move the file pointer where the index starts
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

	// By opening a file pointer everytime we check if a password is pwned, we improve performance
	// by *a lot*. The tradeoff is the cost in CPU cycles.
	//
	// For example, by only having a single file pointer and 500 concurrent requests, the CPU usage
	// (8c/16t) was about 45% but the response time for the requests start going into the 100s of
	// seconds due to the synchronization overhead from multithreaded file access.
	//
	// With a file pointer per request the response times never go above 7s, of course with 100%
	// CPU usage in all cores.
	//
	// This is a solution that works "well enough". I probably should change it to something more CPU
	// efficient. Less power used, less global warming I guess...
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
	// Maybe the computed hash is present exactly as is on the index.
	exact, closest := binarySearch(r.index, h)
	if exact > 0 {
		return true, nil
	}

	// We have to get the closest low value, not the binary latest if the item is not found.
	// This is equivalent to rust's saturating_sub(1) function on binary_search_by_key
	lastEntry := r.index[closest-1]

	reader := newBitReader(file)
	if _, err = reader.Seek(int64(lastEntry.bitPos), io.SeekStart); err != nil {
		return false, err
	}

	// Try to find the probable match from the closest element found in the index.
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
