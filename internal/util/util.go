package util

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"net/http"
	"runtime"
	"strings"
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

func CheckRam(items uint64) {
	required := (items * 8) / (1024 * 1024)
	if memStat, err := mem.VirtualMemory(); err == nil {
		log.Debug().Msgf("System has %.2f MiB of RAM available", float64(memStat.Available)/(1024*1024))
		if required > memStat.Available {
			log.Fatal().Msgf("Your system does not have the minimum required RAM to execute this process.")
		}

		log.Info().Msgf("^C now to stop the process.")
	} else {
		log.Warn().Msgf("Estimated memory use for %d items %d MiB", items, required)
		log.Warn().Msgf("This process will cause disk swapping and general slowness if your "+
			"current system memory is not at least %d MiB. ^C now to stop the process.", required)
	}
}

func CheckDiskSpace(fileName string, sizeGb int) {
	warn := false
	if parts, err := disk.Partitions(false); err == nil {
		for _, part := range parts {
			if strings.Index(fileName, part.Mountpoint) >= 0 {
				if usage, err := disk.Usage(part.Mountpoint); err == nil {
					warn = false
					log.Debug().Msgf("%s has %.2f GiB free", part.Mountpoint, float64(usage.Free)/(1024*1024*1024))
					// 40 GiB
					required := uint64(sizeGb * 1024 * 1024 * 1024)
					if required > usage.Free {
						log.Fatal().Msgf("Drive %s does not have sufficient space free (%d GB) for the download. Please free some space before trying again", part.Mountpoint, sizeGb)
					}
				} else {
					log.Debug().Err(err).Msgf("Error getting current storage sizes")
				}
			}
		}
	} else {
		log.Debug().Err(err).Msgf("Error getting current storage sizes")
	}

	if warn {
		log.Warn().Msgf("IMPORTANT: The haveibeenpwned password file is very large, please ensure you have at least 40GiB free for the download.")
	}
}
