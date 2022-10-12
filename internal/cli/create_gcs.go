package cli

import (
	"bufio"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"io"
	"math"
	"os"
	"pwd-checker/internal/gcs"
	"runtime"
	"sync"
	"time"
)

var (
	outFile          string
	probability      uint64
	indexGranularity uint64

	createCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a Pwned Passwords GCS database from file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
)

func init() {
	createCmd.Flags().StringVarP(&inputFile, "file", "f", "", "Input file path (required)")
	createCmd.MarkFlagRequired("file")
	createCmd.Flags().StringVarP(&outFile, "out", "o", "", "Output file path (required)")
	createCmd.MarkFlagRequired("out")
	createCmd.Flags().Uint64VarP(&probability, "false-positive-rate", "p", 16777216, "False positive rate for queries, 1-in-p.")
	createCmd.Flags().Uint64VarP(&indexGranularity, "index-granularity", "i", 1024, "Entries per index point (16 bytes each).")

	rootCmd.AddCommand(createCmd)
}

// Inspired by https://marcellanz.com/post/file-read-challenge/
func createCommand() error {
	if verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	s := stats()
	defer s()

	file, err := os.Open(inputFile)
	if err != nil {
		return err
	}

	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing Pwned Passwords file")
		}
	}(file)

	out, err := os.Create(outFile)
	if err != nil {
		return err
	}

	defer func(out *os.File) {
		if err = out.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing GCS file")
		}
	}(out)

	// Estimate the amount of lines in the passwords file. It's pretty accurate, <= 1% error rate
	n := estimateFileLines(file)

	log.Info().Msgf("Estimated memory use for %d items %d MiB", n, (n*8)/(1024*1024))
	if n*8 > 1024*1024*1024*2 {
		log.Warn().Msgf("This process will cause disk swapping if your current system memory is not the minimum amount (%d MiB). ^C now to stop the process", (n*8)/(1024*1024))
		time.Sleep(5 * time.Second)
	}

	log.Info().Msg("Converting pwned passwords file. This might take a while")

	// TODO It's all wrong. The encoding, the writing, everything is wrong!
	// For now use Freaky's gcstool

	// 847223402 is the exact number of lines for v8 file
	builder := gcs.NewBuilder(out, n, probability, indexGranularity)
	scanner := bufio.NewScanner(file)
	stat := gcs.NewStatus()

	// Pool to store the read lines from the file, in 64k chunks
	linesChunkLen := 64 * 1024
	linesPool := sync.Pool{New: func() interface{} {
		lines := make([][]byte, 0, linesChunkLen)
		return lines
	}}
	lines := linesPool.Get().([][]byte)[:0]

	recordsPool := sync.Pool{New: func() interface{} {
		entries := make([]uint64, 0, linesChunkLen)
		return entries
	}}

	// Mutex needed to avoid resource contention between the coroutines
	mutex := &sync.Mutex{}
	wg := sync.WaitGroup{}

	stat.StageWork("Hashing", n)
	// Read first line
	scanner.Scan()
	for {
		lines = append(lines, scanner.Bytes())
		willScan := scanner.Scan()

		if len(lines) == linesChunkLen || !willScan {
			linesToProcess := lines
			wg.Add(len(linesToProcess))

			go func() {
				// Clear data
				records := recordsPool.Get().([]uint64)[:0]

				for _, line := range linesToProcess {
					if len(line) < 16 {
						log.Trace().Msgf("Skipping line %s", string(line))
					} else {
						hash := gcs.U64FromHex(line[0:16])
						records = append(records, hash)
					}
				}

				linesPool.Put(linesToProcess)
				// Avoid resource contention
				mutex.Lock()

				for _, hash := range records {
					stat.Incr()
					builder.Add(hash)
				}

				mutex.Unlock()
				recordsPool.Put(records)

				wg.Add(-len(records))
			}()

			// Clear slice
			lines = linesPool.Get().([][]byte)[:0]
		}

		if !willScan {
			break
		}
	}
	// Wait for all coroutines to finish
	wg.Wait()

	// Create the GCS file
	if err = builder.Finalize(stat); err != nil {
		return err
	}

	stat.Done()
	return nil
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

func stats() func() {
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
