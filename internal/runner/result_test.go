package runner

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResultJSONRoundtrip(t *testing.T) {
	started := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	finished := started.Add(5 * time.Second)
	original := Result{
		ID:         "dns-abc123",
		Command:    "python3",
		Args:       []string{"/path/to/dns_lookup.py", "example.com"},
		StartedAt:  started,
		FinishedAt: finished,
		DurationMs: 5000,
		ExitCode:   0,
		Status:     StatusSuccess,
		Stdout:     "A 93.184.216.34\n",
		Stderr:     "",
		Error:      "",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var restored Result
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if restored.ID != original.ID {
		t.Errorf("ID: got %q, want %q", restored.ID, original.ID)
	}
	if restored.Command != original.Command {
		t.Errorf("Command: got %q, want %q", restored.Command, original.Command)
	}
	if len(restored.Args) != len(original.Args) {
		t.Errorf("Args length: got %d, want %d", len(restored.Args), len(original.Args))
	}
	if restored.DurationMs != original.DurationMs {
		t.Errorf("DurationMs: got %d, want %d", restored.DurationMs, original.DurationMs)
	}
	if restored.ExitCode != original.ExitCode {
		t.Errorf("ExitCode: got %d, want %d", restored.ExitCode, original.ExitCode)
	}
	if restored.Status != original.Status {
		t.Errorf("Status: got %q, want %q", restored.Status, original.Status)
	}
	if restored.Stdout != original.Stdout {
		t.Errorf("Stdout: got %q, want %q", restored.Stdout, original.Stdout)
	}
	if restored.Error != original.Error {
		t.Errorf("Error: got %q, want %q", restored.Error, original.Error)
	}
	if !restored.StartedAt.Equal(original.StartedAt) {
		t.Errorf("StartedAt: got %v, want %v", restored.StartedAt, original.StartedAt)
	}
	if !restored.FinishedAt.Equal(original.FinishedAt) {
		t.Errorf("FinishedAt: got %v, want %v", restored.FinishedAt, original.FinishedAt)
	}
}

func TestResultStatusConsts(t *testing.T) {
	if string(StatusSuccess) != "success" {
		t.Errorf("StatusSuccess: got %q, want %q", StatusSuccess, "success")
	}
	if string(StatusFailed) != "failed" {
		t.Errorf("StatusFailed: got %q, want %q", StatusFailed, "failed")
	}
}

func TestResultEmptyFields(t *testing.T) {
	original := Result{}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal empty result failed: %v", err)
	}
	if string(data) == "" {
		t.Error("expected non-empty JSON for empty result")
	}

	var restored Result
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal empty result failed: %v", err)
	}

	if restored.ID != "" {
		t.Errorf("ID: got %q, want empty", restored.ID)
	}
	if restored.Status != "" {
		t.Errorf("Status: got %q, want empty", restored.Status)
	}
}

func TestResultErrorOmitEmpty(t *testing.T) {
	// Error field with omitempty should be absent from JSON when empty,
	// but present when non-empty.
	r := Result{ID: "test-1", Status: StatusSuccess, Error: ""}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	// Unmarshal back and verify Error is still empty.
	var restored Result
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if restored.Error != "" {
		t.Errorf("Error: got %q, want empty (omitempty)", restored.Error)
	}

	// With non-empty Error, it should roundtrip.
	r.Error = "something failed"
	data, err = json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal with error failed: %v", err)
	}
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal with error failed: %v", err)
	}
	if restored.Error != "something failed" {
		t.Errorf("Error: got %q, want %q", restored.Error, "something failed")
	}
}
