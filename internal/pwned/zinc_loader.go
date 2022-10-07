package pwned

import (
	"bufio"
	"encoding/json"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"pwd-checker/internal/config"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
)

type record struct {
	Hash  string `json:"hash"`
	Count string `json:"count"`
}

type bulk struct {
	Index   string   `json:"index"`
	Records []record `json:"records"`
}

// loaderClient we have to use a special client for the bulk loading
func loaderClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxConnsPerHost = 100
	t.MaxIdleConnsPerHost = 100

	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: t,
	}
}

func LoadPwnedPasswordsZinc(fileName string, config config.Config) error {
	log.Info().Msg("Loading pwned passwords database. This might take a while")
	s := stats()
	defer s()

	file, err := os.Open(fileName)
	if err != nil {
		return err
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Error().Msg("Error closing Pwned Passwords File")
		}
	}(file)

	fileLineCount := int64(0)
	scanner := bufio.NewScanner(file)

	// Pool to store the read lines from the file. 512k chunks
	linesChunkLen := 64 * 1024
	linesPool := sync.Pool{New: func() interface{} {
		lines := make([]string, 0, linesChunkLen)
		return lines
	}}
	lines := linesPool.Get().([]string)[:0]

	recordsPool := sync.Pool{New: func() interface{} {
		entries := make([]record, 0, linesChunkLen)
		return entries
	}}

	jsonPool := sync.Pool{New: func() interface{} {
		entries := make([]byte, 0, linesChunkLen)
		return entries
	}}

	mutex := &sync.Mutex{}
	wg := sync.WaitGroup{}

	// Init api client
	//client := zincApi(config)

	// Read first line
	scanner.Scan()
	for {
		lines = append(lines, scanner.Text())
		willScan := scanner.Scan()

		if len(lines) == linesChunkLen || !willScan {
			linesToProcess := lines
			wg.Add(len(linesToProcess))

			go func() {
				atomic.AddInt64(&fileLineCount, int64(len(linesToProcess)))
				// Clear data
				records := recordsPool.Get().([]record)[:0]

				for _, text := range linesToProcess {
					// Clean invisible chars from line
					clean := strings.Map(func(r rune) rune {
						if unicode.IsGraphic(r) {
							return r
						}

						return -1
					}, text)

					pwdSlice := strings.SplitN(clean, ":", 2)
					records = append(records, record{Hash: pwdSlice[0], Count: pwdSlice[1]})
				}

				linesPool.Put(linesToProcess)
				mutex.Lock()

				jsonBytes := jsonPool.Get().([]byte)[:0]
				jsonBytes, _ = json.Marshal(bulk{Index: "Pwned", Records: records})

				//if err == nil {
				//	//fmt.Println(string(jsonBytes))
				//	req, err := http.NewRequest("POST", config.ZincUrl+"/api/_bulkv2", bytes.NewReader(jsonBytes))
				//	if err == nil {
				//		req.Header.Set("content-type", "application/json")
				//		req.Header.Set("authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(config.ZincUser+":"+config.ZincPassword)))
				//
				//		// Call api here
				//		//client := &http.Client{}
				//		//res, err := client.Do(req)
				//		//if err != nil {
				//		//	log.Error().Err(err).Msgf("Error calling zinc load endpoint")
				//		//} else {
				//		//	defer func(Body io.ReadCloser) {
				//		//		_ = Body.Close()
				//		//	}(res.Body)
				//		//
				//		//	if res.StatusCode != http.StatusOK {
				//		//		log.Error().Msgf("Error response with status code %d", res.StatusCode)
				//		//	}
				//		//}
				//		jsonPool.Put(jsonBytes)
				//	} else {
				//		log.Error().Err(err).Msgf("Error creating request for zinc load endpoint")
				//	}
				//} else {
				//	log.Error().Err(err).Msgf("Error marshalling json for bulk loading")
				//}

				mutex.Unlock()
				recordsPool.Put(records)
				jsonPool.Put(jsonBytes)

				wg.Add(-len(records))
			}()

			// Clear slice
			lines = linesPool.Get().([]string)[:0]
		}

		if !willScan {
			break
		}
	}

	wg.Wait()

	log.Debug().Msgf("Loading pwned passwords database done")
	log.Debug().Msgf("Total file line count: %v", fileLineCount)
	return nil
}

func stats() func() {
	start := time.Now()
	return func() {
		log.Debug().Msgf("time to run %v", time.Since(start))
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		log.Debug().Msgf("Alloc: %d MB, TotalAlloc: %d MB, Sys: %d MB",
			ms.Alloc/1024/1024, ms.TotalAlloc/1024/1024, ms.Sys/1024/1024)
		log.Debug().Msgf("Mallocs: %d, Frees: %d",
			ms.Mallocs, ms.Frees)
		log.Debug().Msgf("HeapAlloc: %d MB, HeapSys: %d MB, HeapIdle: %d MB",
			ms.HeapAlloc/1024/1024, ms.HeapSys/1024/1024, ms.HeapIdle/1024/1024)
		log.Debug().Msgf("HeapObjects: %d", ms.HeapObjects)
	}
}
