package runner

import "time"

type Status string

const (
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
)

// Result is a JSON-friendly schema for job outputs.
type Result struct {
	ID         string    `json:"id"`
	Command    string    `json:"command"`
	Args       []string  `json:"args"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	DurationMs int64     `json:"duration_ms"`
	ExitCode   int       `json:"exit_code"`
	Status     Status    `json:"status"`
	Stdout     string    `json:"stdout"`
	Stderr     string    `json:"stderr"`
	Error      string    `json:"error,omitempty"`
}
