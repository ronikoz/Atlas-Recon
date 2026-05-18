package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureOutput runs fn and captures stdout/stderr. Returns exit code and output.
func captureOutput(fn func() int) (exitCode int, stdout, stderr string) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	rOut, wOut, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	os.Stdout = wOut
	os.Stderr = wErr

	outCh := make(chan string)
	errCh := make(chan string)

	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, rOut)
		outCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, rErr)
		errCh <- buf.String()
	}()

	exitCode = fn()

	wOut.Close()
	wErr.Close()
	stdout = <-outCh
	stderr = <-errCh

	return
}

// tempConfig writes a minimal YAML config to a temp file, sets CT_CONFIG to point
// to it, and returns the temp dir path for cleanup.
func tempConfig(t *testing.T, storageEnabled bool, resultsDB string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	lines := []string{
		"concurrency: 4",
		"timeouts:",
		"  command_seconds: 120",
		"storage:",
	}
	if storageEnabled {
		lines = append(lines,
			"  enabled: true",
			"  results_db: "+resultsDB,
			"  max_records: 100",
		)
	} else {
		lines = append(lines,
			"  enabled: false",
		)
	}
	lines = append(lines,
		"paths:",
		"  python: python3",
		"  whois: whois",
	)

	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CT_CONFIG", path)
	return dir
}

// ---------------------------------------------------------------------------
// 1.1: CLI contract basics
// ---------------------------------------------------------------------------

func TestRootHelp(t *testing.T) {
	exitCode, stdout, _ := captureOutput(func() int {
		return Execute([]string{"ct", "--help"})
	})
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "Atlas-Recon") {
		t.Errorf("expected output to contain 'Atlas-Recon', got: %s", stdout)
	}
}

func TestScanHelp(t *testing.T) {
	_, stdout, _ := captureOutput(func() int {
		return Execute([]string{"ct", "scan", "--help"})
	})
	if !strings.Contains(stdout, "--ports") {
		t.Errorf("expected --ports flag in scan help, got: %s", stdout)
	}
}

func TestResultsHelp(t *testing.T) {
	_, stdout, _ := captureOutput(func() int {
		return Execute([]string{"ct", "results", "--help"})
	})
	if !strings.Contains(stdout, "--clear") {
		t.Errorf("expected --clear flag in results help, got: %s", stdout)
	}
	if !strings.Contains(stdout, "--older-than") {
		t.Errorf("expected --older-than flag in results help, got: %s", stdout)
	}
}

func TestDNSHelp(t *testing.T) {
	_, stdout, _ := captureOutput(func() int {
		return Execute([]string{"ct", "dns", "--help"})
	})
	if !strings.Contains(stdout, "--targets-file") {
		t.Errorf("expected --targets-file flag in dns help, got: %s", stdout)
	}
}

func TestJSONPersistentFlag(t *testing.T) {
	// --json is a persistent flag. Verify it appears in subcommand help.
	_, stdout, _ := captureOutput(func() int {
		return Execute([]string{"ct", "scan", "--help"})
	})
	if !strings.Contains(stdout, "--json") {
		t.Errorf("expected --json persistent flag in scan help, got: %s", stdout)
	}
}

func TestBadConfigPath(t *testing.T) {
	exitCode, _, stderr := captureOutput(func() int {
		// Must provide a valid subcommand + arg so cobra arg validation passes
		// before PersistentPreRunE runs config loading.
		return Execute([]string{"ct", "--config", "/nonexistent/path.yaml", "scan", "localhost"})
	})
	if exitCode == 0 {
		t.Error("expected non-zero exit code for bad config path")
	}
	if !strings.Contains(stderr, "nonexistent") {
		t.Errorf("expected stderr to mention bad path, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// 1.2: Scan command contracts
// ---------------------------------------------------------------------------

func TestScanDefaultPorts(t *testing.T) {
	// Isolate from real config — disable storage to avoid touching real db.
	_ = tempConfig(t, false, "")
	exitCode, stdout, _ := captureOutput(func() int {
		return Execute([]string{"ct", "scan", "localhost"})
	})
	// Scan should complete without error even if no ports are open.
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for scan, got %d", exitCode)
	}
	// Output should contain scan header.
	if !strings.Contains(stdout, "Scan Results") {
		t.Errorf("expected 'Scan Results' in output, got: %s", stdout)
	}
}

func TestScanJSONOutput(t *testing.T) {
	_ = tempConfig(t, false, "")
	exitCode, stdout, _ := captureOutput(func() int {
		return Execute([]string{"ct", "scan", "localhost", "--ports", "80", "--json"})
	})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for scan --json, got %d", exitCode)
	}

	// Verify valid JSON with required fields.
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("expected valid JSON, got error: %v\nstdout: %s", err, stdout)
	}
	for _, field := range []string{"host", "ports", "start_time", "end_time"} {
		if _, ok := result[field]; !ok {
			t.Errorf("expected field %q in scan JSON, got keys: %v", field, mapKeys(result))
		}
	}
	if result["host"] != "localhost" {
		t.Errorf("expected host 'localhost', got %v", result["host"])
	}
}

func TestScanInvalidPorts(t *testing.T) {
	_ = tempConfig(t, false, "")
	exitCode, _, stderr := captureOutput(func() int {
		return Execute([]string{"ct", "scan", "localhost", "--ports", "abc"})
	})
	if exitCode == 0 {
		t.Error("expected non-zero exit code for invalid ports")
	}
	combined := stderr
	if !strings.Contains(combined, "error parsing ports") {
		t.Errorf("expected 'error parsing ports' in output, got: %q", combined)
	}
}

// ---------------------------------------------------------------------------
// 1.3: Results command contracts
// ---------------------------------------------------------------------------

func TestResultsClear(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "results.db")
	tempConfig(t, true, dbPath)

	exitCode, stdout, _ := captureOutput(func() int {
		return Execute([]string{"ct", "results", "--clear"})
	})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for results --clear, got %d", exitCode)
	}
	if !strings.Contains(stdout, "Deleted") {
		t.Errorf("expected 'Deleted' in output, got: %s", stdout)
	}
}

func TestResultsOlderThan(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "results.db")
	tempConfig(t, true, dbPath)

	exitCode, stdout, _ := captureOutput(func() int {
		return Execute([]string{"ct", "results", "--older-than", "30d"})
	})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for results --older-than, got %d", exitCode)
	}
	if !strings.Contains(stdout, "Pruned") {
		t.Errorf("expected 'Pruned' in output, got: %s", stdout)
	}
}

func TestResultsInvalidDuration(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "results.db")
	tempConfig(t, true, dbPath)

	exitCode, _, stderr := captureOutput(func() int {
		return Execute([]string{"ct", "results", "--older-than", "xxx"})
	})
	if exitCode == 0 {
		t.Error("expected non-zero exit code for invalid duration")
	}
	if !strings.Contains(stderr, "invalid duration") || !strings.Contains(stderr, "xxx") {
		t.Errorf("expected 'invalid duration' in stderr, got: %q", stderr)
	}
}

func TestResultsJSON(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "results.db")
	tempConfig(t, true, dbPath)

	exitCode, stdout, _ := captureOutput(func() int {
		return Execute([]string{"ct", "results", "--json"})
	})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for results --json, got %d", exitCode)
	}

	// Should be a valid JSON array (empty when no records exist).
	trimmed := strings.TrimSpace(stdout)
	var result []interface{}
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		t.Fatalf("expected valid JSON array, got error: %v\nstdout: %s", err, stdout)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func mapKeys(m map[string]interface{}) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
