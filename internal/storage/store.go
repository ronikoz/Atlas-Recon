package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS results (
	id TEXT PRIMARY KEY,
	kind TEXT NOT NULL,
	command TEXT NOT NULL,
	args TEXT NOT NULL,
	started_at TEXT NOT NULL,
	finished_at TEXT NOT NULL,
	duration_ms INTEGER NOT NULL,
	exit_code INTEGER NOT NULL,
	status TEXT NOT NULL,
	stdout TEXT NOT NULL,
	stderr TEXT NOT NULL,
	error TEXT NOT NULL,
	payload TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS results_command_idx ON results(command);
CREATE INDEX IF NOT EXISTS results_started_idx ON results(started_at);
`

type Store struct {
	db *sql.DB
}

type Record struct {
	ID         string
	Kind       string
	Command    string
	Args       []string
	StartedAt  time.Time
	FinishedAt time.Time
	DurationMs int64
	ExitCode   int
	Status     string
	Stdout     string
	Stderr     string
	Error      string
	Payload    string
}

type ListOptions struct {
	Limit   int
	Command string
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("results db path is empty")
	}
	if err := ensureDir(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := initSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) SaveRecord(record Record) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	argsJSON, err := json.Marshal(record.Args)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO results
			(id, kind, command, args, started_at, finished_at, duration_ms, exit_code, status, stdout, stderr, error, payload)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID,
		record.Kind,
		record.Command,
		string(argsJSON),
		record.StartedAt.UTC().Format(time.RFC3339Nano),
		record.FinishedAt.UTC().Format(time.RFC3339Nano),
		record.DurationMs,
		record.ExitCode,
		record.Status,
		record.Stdout,
		record.Stderr,
		record.Error,
		record.Payload,
	)
	return err
}

func (s *Store) ListRecords(opts ListOptions) ([]Record, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store not initialized")
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	query := `SELECT id, kind, command, args, started_at, finished_at, duration_ms, exit_code, status, stdout, stderr, error, payload
		FROM results`
	var args []any
	if opts.Command != "" {
		query += " WHERE command = ?"
		args = append(args, opts.Command)
	}
	query += " ORDER BY started_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var record Record
		var argsJSON string
		var startedAt string
		var finishedAt string
		if err := rows.Scan(
			&record.ID,
			&record.Kind,
			&record.Command,
			&argsJSON,
			&startedAt,
			&finishedAt,
			&record.DurationMs,
			&record.ExitCode,
			&record.Status,
			&record.Stdout,
			&record.Stderr,
			&record.Error,
			&record.Payload,
		); err != nil {
			return nil, err
		}
		record.StartedAt = parseTime(startedAt)
		record.FinishedAt = parseTime(finishedAt)
		if argsJSON != "" {
			_ = json.Unmarshal([]byte(argsJSON), &record.Args)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func initSchema(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, schema)
	return err
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
