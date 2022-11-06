package hibp

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog/log"
	"github.com/thinhdanggroup/executor"
	"golang.org/x/net/context"
	"io"
	"net"
	"net/http"
	"os"
	"pwd-checker/internal/util"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Downloader struct {
	parallelism int
	stat        *status
	wm          sync.Mutex
	fileName    string
	writer      *bufio.Writer
	http        *retryablehttp.Client
}

func NewDownloader(out *os.File, parallelism int) *Downloader {
	return &Downloader{
		parallelism: parallelism,
		writer:      bufio.NewWriter(out),
		http:        initHttpClient(),
		fileName:    out.Name(),
	}
}

func initHttpClient() *retryablehttp.Client {
	client := retryablehttp.NewClient()
	// Too much garbage in the logs, it slowed the download too much.
	client.Logger = nil

	// Retry Max 10 times on protocol errors. Any other are just reported and not retried.
	client.RetryMax = 10

	client.HTTPClient = &http.Client{
		Transport: &http.Transport{
			DisableCompression: false,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS13,
			},
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       10 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			// HTTP/2 is "better" but only establishes one connection. Disabling it is much faster.
			// It also introduced some read errors when getting the responses, so HTTP/1.1 it is.
			ForceAttemptHTTP2:   false,
			MaxIdleConnsPerHost: runtime.GOMAXPROCS(0) + 1,
		},
	}

	return client
}

func (d *Downloader) ProcessRanges(ranges int, skipWait bool) error {
	util.CheckDiskSpace(d.fileName, 40)

	s := util.Stats()
	defer s()

	var threads int
	if d.parallelism > 0 {
		threads = d.parallelism
	} else {
		// About 8 times nets me a sustained download of about 150 Mbit/s (96 threads), so it seems
		// like a good default to set
		threads = runtime.NumCPU() * 8
	}

	// This is a bounded thread pool. I just didn't want to implement it myself...
	downloadTasks, err := executor.New(executor.Config{
		ReqPerSeconds: 0,
		QueueSize:     2 * threads,
		NumWorkers:    threads,
	})
	if err != nil {
		return err
	}
	defer downloadTasks.Close()

	log.Info().Msgf("download Pwned Passwords SHA1 Hashes in file %s with %d threads, ^C to stop the process", d.fileName, threads)
	if !skipWait {
		time.Sleep(10 * time.Second)
	}
	log.Info().Msg("starting process. This might take a while, be patient :)")
	d.stat = newStatus(ranges)
	d.stat.BeginProgress()

	// Start downloading ranges concurrently from 00000 to FFFFF
	for i := 0; i < ranges; i++ {
		prefix := getHashRange(i)
		if err = downloadTasks.Publish(d.ProcessRange, prefix); err != nil {
			log.Panic().Err(err).Msgf("there is a programming error here.")
		}
	}

	downloadTasks.Wait()
	d.stat.Done()

	if f, err := os.Stat(d.fileName); err == nil {
		log.Debug().Msgf("file %s is %.2fGiB", d.fileName, float64(f.Size())/(1024*1024*1024))
	}
	return nil
}

func getHashRange(i int) string {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(i))
	// First 5 characters, k-anonymity needs the hash like this
	return strings.ToUpper(hex.EncodeToString(buf)[3:])
}

func rangeHttpRequest(prefix string) (*retryablehttp.Request, error) {
	ctx := context.WithValue(context.Background(), "range", prefix)
	req, err := retryablehttp.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://api.pwnedpasswords.com/range/%s", prefix),
		nil,
	)
	if err != nil {
		return nil, err
	}
	// This user agent string is identifying enough, I think...
	req.Header.Set("User-Agent", "golang-hibp-downloader/1.0")
	return req, nil
}

func (d *Downloader) ProcessRange(prefix string) {
	if data, err := d.downloadRange(prefix); err == nil {
		if err = d.writeRangeToFile(prefix, data); err == nil {
			d.stat.RangeDownloaded()
		} else {
			log.Fatal().Err(err).Msgf("error during file write for range %s. Stopping process", prefix)
		}
	} else {
		log.Error().Err(err).Msgf("error downloading range %s", prefix)
	}
}

func (d *Downloader) downloadRange(prefix string) ([]byte, error) {
	timer := time.Now()
	req, err := rangeHttpRequest(prefix)
	if err != nil {
		return nil, err
	}

	res, err := d.http.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode < 400 {
		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		defer func(Body io.ReadCloser) {
			err = Body.Close()
			if err != nil {
				log.Warn().Err(err).Msgf("error closing body for range %s", prefix)
			}
		}(res.Body)

		d.stat.RequestComplete(res, time.Since(timer).Milliseconds())
		return resBody, nil
	}

	return nil, fmt.Errorf("request [%s] failed with status [%d] %s", req.RequestURI, res.StatusCode, res.Status)
}

func (d *Downloader) writeRangeToFile(prefix string, r []byte) error {
	// Synchronize file writes, we don't want intersected or incomplete lines written to the file.
	d.wm.Lock()
	defer d.wm.Unlock()

	scanner := bufio.NewScanner(bytes.NewReader(r))
	for scanner.Scan() {
		line := fmt.Sprintf("%s%s\r\n", prefix, scanner.Text())
		if _, err := d.writer.WriteString(line); err != nil {
			return err
		}
		d.stat.HashDownloaded()
	}

	if err := d.writer.Flush(); err != nil {
		return err
	}

	return nil
}
