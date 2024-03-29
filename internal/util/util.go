// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package util

import (
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"regexp"
	"runtime"
	"strings"
	"time"
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

func CheckRam(items uint64, skipWait bool) {
	required := (items * 8) / (1024 * 1024)
	if memStat, err := mem.VirtualMemory(); err == nil {
		log.Debug().Msgf("system has %.2f MiB of RAM available", float64(memStat.Available)/(1024*1024))
		if required > memStat.Available {
			log.Fatal().Msgf("your system does not have the minimum required RAM to execute this process.")
		}

		log.Info().Msgf("^C now to stop the process.")
		if !skipWait {
			time.Sleep(10 * time.Second)
		}
	} else {
		log.Warn().Msgf("estimated memory use for %d items %d MiB", items, required)
		log.Warn().Msgf("this process will cause disk swapping and general slowness if your "+
			"current system memory is not at least %d MiB. ^C now to stop the process.", required)
		if !skipWait {
			time.Sleep(10 * time.Second)
		}
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
						log.Fatal().Msgf("drive %s does not have sufficient space free (%d GB) for the download. Please free some space before trying again", part.Mountpoint, sizeGb)
					}
				} else {
					log.Debug().Err(err).Msgf("error getting current storage sizes")
				}
			}
		}
	} else {
		log.Debug().Err(err).Msgf("error getting current storage sizes")
	}

	if warn {
		log.Warn().Msgf("IMPORTANT: The haveibeenpwned password file is very large, please ensure you have at least 40GiB free for the download.")
	}
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func ToScreamingSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToUpper(snake)
}
