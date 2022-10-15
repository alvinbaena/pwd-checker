package gcs

import (
	"encoding/binary"
	"github.com/rs/zerolog/log"
	"io"
	"math"
	"os"
	"strconv"
)

func U64FromHex(src []byte) uint64 {
	result := uint64(0)
	for _, c := range src {
		result = result * 16
		v, err := strconv.ParseUint(string(c), 16, 64)
		if err != nil {
			log.Fatal().Err(err).Msg("Possible integer overflow?")
		}
		result += v
	}

	return result
}

func estimateFileLines(f *os.File) uint64 {
	// 16MiB
	const EstimateLimit = 1024 * 1024 * 16

	info, err := f.Stat()
	if err != nil {
		log.Fatal().Err(err).Msg("Error estimating lines of file")
	}

	size := info.Size()
	sampleSize := math.Min(float64(size), EstimateLimit)
	buffer := make([]byte, int64(sampleSize))
	if _, err = f.Read(buffer); err != nil {
		log.Fatal().Err(err).Msg("Error estimating lines of file")
	}
	// Reset the file pointer to the start of the file so the actual read will not be missing a
	// 16 MiB chunk
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		log.Fatal().Err(err).Msg("Error estimating lines of file")
	}

	// Count the amount of \n present in buffer
	ascii := []byte("\n")[0]
	sample := 0
	for _, b := range buffer {
		if b == ascii {
			sample++
		}
	}

	return uint64(sample) * (uint64(size) / uint64(sampleSize))
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
