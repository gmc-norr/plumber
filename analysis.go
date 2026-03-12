package plumber

import "github.com/google/uuid"

type AnalysisState string

const (
	StatePending AnalysisState = "pending"
	StateRunning AnalysisState = "running"
	StateFailed  AnalysisState = "failed"
	StateSuccess AnalysisState = "success"
)

type Analysis struct {
	Id       uuid.UUID
	Pipeline Pipeline
	Workdir  string
	State    AnalysisState
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
	a.State = state
	return a
}
