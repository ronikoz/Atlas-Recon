package runner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ronikoz/atlas-recon/internal/plugins"
)

const defaultPython = "python3"

type RunOptions struct {
	Stream       bool
	Python       string
	Timeout      time.Duration
	Context      context.Context
	APIKeys      map[string]string
	LineCallback func(line string) // when set, stdout is delivered line-by-line via callback
}

// RunPython executes a python plugin script and streams output to the console.
// If scriptPath is a relative path, it will attempt to locate the plugin from
// the embedded filesystem first, then fall back to the filesystem.
func RunPython(scriptPath string, args []string, opts RunOptions) (Result, error) {
	// Use configured Python, or fall back to the project's venv.
	// If the venv cannot be found or isn't created yet, fall back to default system python3.
	python := opts.Python
	if python == "" {
		python = os.Getenv("CT_PYTHON")
	}
	if python == "" || python == defaultPython {
		venvPython := GetVenvPython()
		if venvPython != "" {
			if _, err := os.Stat(venvPython); err == nil {
				python = venvPython
			}
		}
	}
	if python == "" {
		python = defaultPython
	}

	// Try to resolve plugin path
	resolvedPath, pluginErr := plugins.GetPluginPath(scriptPath)
	if pluginErr == nil {
		scriptPath = resolvedPath
	} else if _, statErr := os.Stat(scriptPath); statErr != nil {
		// If plugin not found through either method, return error
		return Result{}, fmt.Errorf("plugin not found: %s (%v)", scriptPath, pluginErr)
	}

	ctx := context.Background()
	if opts.Context != nil {
		ctx = opts.Context
	}
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, python, append([]string{scriptPath}, args...)...)
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	// Build env once, used by both paths.
	env := append(os.Environ(), "FORCE_COLOR=1", "CLICOLOR_FORCE=1")
	if opts.APIKeys != nil {
		for k, v := range opts.APIKeys {
			if v != "" {
				env = append(env, fmt.Sprintf("CT_API_%s=%s", strings.ToUpper(k), v))
			}
		}
	}
	cmd.Env = env

	if opts.LineCallback != nil {
		pr, pw, err := os.Pipe()
		if err != nil {
			return Result{}, fmt.Errorf("pipe error: %w", err)
		}
		cmd.Stdout = pw
		cmd.Stderr = &stderrBuf
		cmd.Stdin = os.Stdin

		started := time.Now()
		if err := cmd.Start(); err != nil {
			pw.Close()
			pr.Close()
			return Result{}, fmt.Errorf("python runner start failed: %w", err)
		}
		pw.Close() // close write end so scanner sees EOF when process exits

		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			stdoutBuf.WriteString(line + "\n")
			opts.LineCallback(line)
		}
		pr.Close()

		runErr := cmd.Wait()
		finished := time.Now()

		result := Result{
			Command:    python,
			Args:       append([]string{scriptPath}, args...),
			StartedAt:  started,
			FinishedAt: finished,
			DurationMs: finished.Sub(started).Milliseconds(),
			ExitCode:   exitCode(runErr),
			Stdout:     stdoutBuf.String(),
			Stderr:     stderrBuf.String(),
		}
		if runErr != nil {
			result.Status = StatusFailed
			result.Error = runErr.Error()
			return result, fmt.Errorf("python runner failed: %w", runErr)
		}
		result.Status = StatusSuccess
		return result, nil
	}

	if opts.Stream {
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	} else {
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
	}
	cmd.Stdin = os.Stdin

	started := time.Now()
	err := cmd.Run()
	finished := time.Now()

	result := Result{
		Command:    python,
		Args:       append([]string{scriptPath}, args...),
		StartedAt:  started,
		FinishedAt: finished,
		DurationMs: finished.Sub(started).Milliseconds(),
		ExitCode:   exitCode(err),
		Stdout:     stdoutBuf.String(),
		Stderr:     stderrBuf.String(),
	}

	if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		return result, fmt.Errorf("python runner failed: %w", err)
	}

	result.Status = StatusSuccess
	return result, nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return 1
}

// Signed-off-by: ronikoz
