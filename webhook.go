package plumber

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type MarshableError struct {
	error
}

type MessageType int

const (
	MessageInit MessageType = iota
	MessageStart
	MessageProgress
	MessageEnd
)

func (t MessageType) String() string {
	switch t {
	case MessageInit:
		return "init"
	case MessageStart:
		return "start"
	case MessageProgress:
		return "progress"
	case MessageEnd:
		return "end"
	}
	return "undefined"
}

func (t MessageType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

func NewMarshableError(err error) MarshableError {
	return MarshableError{
		error: err,
	}
}

func (err MarshableError) MarshalJSON() ([]byte, error) {
	if err.error == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(err.Error())
}

type WebhookMessage struct {
	AnalysisId      uuid.UUID      `json:"analysis_id"`
	Pipeline        string         `json:"pipeline"`
	PipelineVersion string         `json:"pipeline_version"`
	Workdir         string         `json:"workdir"`
	Message         any            `json:"message"`
	MessageType     MessageType    `json:"message_type"`
	Success         bool           `json:"success"`
	Error           MarshableError `json:"error"`
	Time            time.Time      `json:"time"`
}

type ProgressMessage struct {
	Message string `json:"message"`
	// Elapsed time in seconds
	Elapsed float64 `json:"elapsed"`
}
