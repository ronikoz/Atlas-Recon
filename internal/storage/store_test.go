package storage

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func tmpStore(t *testing.T) *Store {
	t.Helper()
	f, err := os.CreateTemp("", "atlas-recon-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	s, err := Open(f.Name(), 100)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func makeRecord(id string, startedAt time.Time) Record {
	return Record{
		ID:         id,
		Kind:       "command",
		Command:    "test",
		Args:       []string{"arg1"},
		StartedAt:  startedAt,
		FinishedAt: startedAt.Add(time.Second),
		DurationMs: 1000,
		ExitCode:   0,
		Status:     "success",
		Stdout:     "",
		Stderr:     "",
		Error:      "",
		Payload:    "",
	}
}

func TestDeleteAllRecords(t *testing.T) {
	s := tmpStore(t)
	for i := 0; i < 3; i++ {
		if err := s.SaveRecord(makeRecord(fmt.Sprintf("id-%d", i), time.Now())); err != nil {
			t.Fatal(err)
		}
	}
	n, err := s.DeleteAllRecords()
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("expected 3 deleted, got %d", n)
	}
	records, _ := s.ListRecords(ListOptions{Limit: 10})
	if len(records) != 0 {
		t.Errorf("expected 0 records after delete, got %d", len(records))
	}
}

func TestPruneOldRecords(t *testing.T) {
	s := tmpStore(t)
	old := time.Now().Add(-48 * time.Hour)
	if err := s.SaveRecord(makeRecord("old-1", old)); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRecord(makeRecord("new-1", time.Now())); err != nil {
		t.Fatal(err)
	}
	n, err := s.PruneOldRecords(24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 pruned, got %d", n)
	}
	records, _ := s.ListRecords(ListOptions{Limit: 10})
	if len(records) != 1 || records[0].ID != "new-1" {
		t.Errorf("expected only new-1 to remain, got %+v", records)
	}
}

func TestAutoPrune(t *testing.T) {
	// Open with maxRecords=3, insert 5, verify auto-prune keeps most recent
	f, err := os.CreateTemp("", "atlas-recon-autoprune-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	// Insert 5 records into a store with no limit first
	s0, err := Open(f.Name(), 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		ts := time.Now().Add(time.Duration(i) * time.Second)
		if err := s0.SaveRecord(makeRecord(fmt.Sprintf("rec-%d", i), ts)); err != nil {
			t.Fatal(err)
		}
	}
	s0.Close()

	// Reopen with maxRecords=3 — should auto-prune to keep most recent 80% of 3 = 2
	s1, err := Open(f.Name(), 3)
	if err != nil {
		t.Fatal(err)
	}
	defer s1.Close()
	records, _ := s1.ListRecords(ListOptions{Limit: 10})
	// keep = 3 * 80 / 100 = 2, so 2 records should remain
	if len(records) != 2 {
		t.Errorf("expected 2 records after auto-prune (keep=2), got %d", len(records))
	}
}
