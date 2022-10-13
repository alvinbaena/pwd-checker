package gcs

import (
	"github.com/rs/zerolog/log"
	"time"
)

type Status struct {
	stageName  *string
	workCount  uint64
	doneCount  uint64
	step       uint64
	start      time.Time
	stageStart time.Time
}

func NewStatus() *Status {
	return &Status{
		workCount: 0,
		doneCount: 0,
		step:      0,
		start:     time.Now(),
	}
}

func (s *Status) Stage(stage string) {
	s.FinishStage()

	s.stageName = &stage
	s.stageStart = time.Now()
	s.doneCount = 0
}

func (s *Status) SetWork(count uint64) {
	s.workCount = count
	s.step = count / 20
}

func (s *Status) StageWork(name string, work uint64) {
	s.Stage(name)
	s.SetWork(work)
}

func (s *Status) PrintStatus() {
	elapsed := time.Since(s.start)
	log.Info().Msgf(
		"%s: %d of %d, %.2f%%, %.0f/sec",
		*s.stageName,
		s.doneCount,
		s.workCount,
		float64(s.doneCount)/float64(s.workCount)*100,
		float64(s.doneCount)/elapsed.Seconds()+float64(elapsed.Nanoseconds()/1000000000),
	)
}

func (s *Status) SetWorkDone(count uint64) {
	s.doneCount = count
	if s.doneCount%s.step == 0 {
		s.PrintStatus()
	}
}

func (s *Status) AddWork(count uint64) {
	s.doneCount += count
	if s.doneCount%s.step == 0 {
		s.PrintStatus()
	}
}

func (s *Status) Incr() {
	s.AddWork(1)
}

func (s *Status) FinishStage() {
	if s.stageName != nil {
		elapsed := time.Since(s.stageStart)
		log.Info().Msgf("%s complete in %v", *s.stageName, elapsed)
	}

	none := ""
	s.stageName = &none
}

func (s *Status) Done() {
	s.FinishStage()
	elapsed := time.Since(s.start)
	log.Info().Msgf("Complete in %v", elapsed)
}
