package plumber

import (
	"time"

	"github.com/google/uuid"
)

type AnalysisState string

const (
	StatePending AnalysisState = "pending"
	StateRunning AnalysisState = "running"
	StateFailed  AnalysisState = "failed"
	StateSuccess AnalysisState = "success"
)

func (s *AnalysisState) WithTime(t time.Time) TimedAnalysisState {
	return TimedAnalysisState{
		AnalysisState: *s,
		Time:          t,
	}
}

type TimedAnalysisState struct {
	AnalysisState `json:"name"`
	Time          time.Time `json:"time"`
}

type Analysis struct {
	Id       uuid.UUID          `json:"id"`
	Pipeline Pipeline           `json:"pipeline"`
	Workdir  string             `json:"workdir"`
	State    TimedAnalysisState `json:"state"`
}

func NewAnalysis() *Analysis {
	return &Analysis{}
}

func (a *Analysis) WithId(id uuid.UUID) *Analysis {
	a.Id = id
	return a
}

func (a *Analysis) WithPipeline(p Pipeline) *Analysis {
	a.Pipeline = p
	return a
}

func (a *Analysis) WithWorkdir(path string) *Analysis {
	a.Workdir = path
	return a
}

func (a *Analysis) WithState(state AnalysisState) *Analysis {
	a.State = state.WithTime(time.Now())
	return a
}

func (a *Analysis) SetState(state AnalysisState) {
	a.State = TimedAnalysisState{
		AnalysisState: state,
		Time:          time.Now(),
	}
}
