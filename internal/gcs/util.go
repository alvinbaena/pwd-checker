package gcs

import (
	"github.com/rs/zerolog/log"
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
