package hibp

import (
	"github.com/rs/zerolog/log"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"net/http"
	"sync/atomic"
	"time"
)

type status struct {
	rangesDownloaded           uint64
	hashesDownloaded           uint64
	cloudflareRequests         uint64
	cloudflareHits             uint64
	cloudflareMisses           uint64
	cloudflareRequestTimeTotal uint64
	start                      time.Time
	ticker                     *time.Ticker
	progress                   chan bool
	totalRanges                int
}

func newStatus(totalRanges int) *status {
	return &status{
		start:       time.Now(),
		ticker:      time.NewTicker(10 * time.Second),
		progress:    make(chan bool),
		totalRanges: totalRanges,
	}
}

// BeginProgress reports the progress of the download every 10 seconds.
func (s *status) BeginProgress() {
	go func() {
		for {
			select {
			case <-s.progress:
				return
			case <-s.ticker.C:
				total := float64(s.totalRanges)
				log.Info().Msgf("%.2f%% Hash ranges downloaded. %.0f hashes/s", (float64(s.rangesDownloaded)*100)/total, s.hashesPerSecond())
			}
		}
	}()
}

func (s *status) RangeDownloaded() {
	atomic.AddUint64(&s.rangesDownloaded, 1)
}

func (s *status) HashDownloaded() {
	atomic.AddUint64(&s.hashesDownloaded, 1)
}

func (s *status) RequestComplete(res *http.Response, millis int64) {
	atomic.AddUint64(&s.cloudflareRequestTimeTotal, uint64(millis))
	atomic.AddUint64(&s.cloudflareRequests, 1)

	if cacheHit := res.Header.Get("CF-Cache-Status"); cacheHit == "HIT" {
		atomic.AddUint64(&s.cloudflareHits, 1)
	} else {
		atomic.AddUint64(&s.cloudflareMisses, 1)
	}
}

func (s *status) hashesPerSecond() float64 {
	elapsed := time.Since(s.start)
	var hashesPerSec float64
	if elapsed.Nanoseconds() > 0 {
		hashesPerSec = float64(s.hashesDownloaded) / elapsed.Seconds()
	} else {
		hashesPerSec = float64(s.hashesDownloaded)
	}

	return hashesPerSec
}

func (s *status) Done() {
	s.progress <- true
	cloudflareHitPercent := float64(s.cloudflareHits*100) / float64(s.cloudflareRequests)
	cloudflareMissPercent := float64(s.cloudflareMisses*100) / float64(s.cloudflareRequests)
	requestAverage := float64(s.cloudflareRequestTimeTotal) / float64(s.cloudflareRequests)

	p := message.NewPrinter(language.English)
	log.Info().Msgf("finished downloading all hash ranges in %v. %.0f hashes/s", time.Since(s.start), s.hashesPerSecond())
	log.Debug().Msgf("made %s Cloudflare requests. Average response time %.2f ms", p.Sprintf("%d", s.cloudflareRequests), requestAverage)
	log.Debug().Msgf("cloudflare cache hits: %s (%.2f%%), misses: %s (%.2f%%)", p.Sprintf("%d", s.cloudflareHits), cloudflareHitPercent, p.Sprintf("%d", s.cloudflareMisses), cloudflareMissPercent)
}
