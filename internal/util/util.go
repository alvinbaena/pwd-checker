package util

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	"runtime"
)

func Stats() func() {
	return func() {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		log.Debug().Msgf("Alloc: %d MB, TotalAlloc: %d MB, Requested: %d MB",
			ms.Alloc/1024/1024, ms.TotalAlloc/1024/1024, ms.Sys/1024/1024)
		log.Debug().Msgf("Mallocs: %d, Frees: %d, GC: %d", ms.Mallocs, ms.Frees, ms.NumGC)
		log.Debug().Msgf("HeapAlloc: %d MB, HeapSys: %d MB, HeapIdle: %d MB",
			ms.HeapAlloc/1024/1024, ms.HeapSys/1024/1024, ms.HeapIdle/1024/1024)
		log.Debug().Msgf("HeapObjects: %d", ms.HeapObjects)
	}
}

func ApplyCliSettings(verbose bool, profile bool, pprofPort uint16) {
	if verbose {
		log.Warn().Msgf("Verbosity up")
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if profile {
		log.Info().Msgf("Profiling is enabled for this session. Server will listen on port %d", pprofPort)
		go func() {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", pprofPort), nil); err != nil {
				log.Error().Err(err).Msgf("Error starting profiling server on port %d", pprofPort)
				return
			}
		}()
	}
}
