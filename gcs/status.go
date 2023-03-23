// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package gcs

import (
	"github.com/rs/zerolog/log"
	"sync/atomic"
	"time"
)

type status struct {
	stageName  *string
	workCount  uint64
	doneCount  uint64
	step       uint64
	start      time.Time
	stageStart time.Time
}

func newStatus() *status {
	return &status{start: time.Now()}
}

func (s *status) Stage(stage string) {
	s.FinishStage()

	s.stageName = &stage
	log.Info().Msgf("%s starting...", *s.stageName)

	s.stageStart = time.Now()
	s.doneCount = 0
}

func (s *status) SetWork(count uint64) {
	s.workCount = count
	s.step = count / 20
}

func (s *status) StageWork(name string, work uint64) {
	s.Stage(name)
	s.SetWork(work)
}

func (s *status) PrintStatus() {
	elapsed := time.Since(s.start)
	log.Info().Msgf(
		"%s: %d of %d, %.2f%%, %.0f/s",
		*s.stageName,
		s.doneCount,
		s.workCount,
		float64(s.doneCount)/float64(s.workCount)*100,
		float64(s.doneCount)/elapsed.Seconds()+float64(elapsed.Nanoseconds()/1_000_000_000),
	)
}

func (s *status) AddWork(count uint64) {
	atomic.AddUint64(&s.doneCount, count)
	if s.doneCount%s.step == 0 {
		s.PrintStatus()
	}
}

func (s *status) Incr() {
	s.AddWork(1)
}

func (s *status) FinishStage() {
	if s.stageName != nil {
		elapsed := time.Since(s.stageStart)
		log.Info().Msgf("%s complete in %v", *s.stageName, elapsed)
	}

	none := ""
	s.stageName = &none
}

func (s *status) Done() {
	s.FinishStage()
	elapsed := time.Since(s.start)
	log.Info().Msgf("complete in %v", elapsed)
}
