package plumber

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const AnalysisFile string = ".plumber-analysis.json"

type AnalysisState string

const (
	StatePending AnalysisState = "pending"
	StateRunning AnalysisState = "running"
	StateFailed  AnalysisState = "failed"
	StateSuccess AnalysisState = "succeeded"
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
	User     string             `json:"user"`
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

func (a *Analysis) WithUser(user string) *Analysis {
	a.User = user
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

func (a *Analysis) Write() error {
	if a.Workdir == "" {
		return fmt.Errorf("missing workdir")
	}
	f, err := os.Create(filepath.Join(a.Workdir, AnalysisFile))
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	return err
}

func (a *Analysis) Read() (Analysis, error) {
	var analysis Analysis
	if a.Workdir == "" {
		return analysis, fmt.Errorf("missing workdir")
	}
	f, err := os.Open(filepath.Join(a.Workdir, AnalysisFile))
	if err != nil {
		return analysis, err
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return analysis, err
	}
	err = json.Unmarshal(b, &analysis)
	return analysis, err
}
