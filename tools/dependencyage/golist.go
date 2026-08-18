package dependencyage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const stderrTailLimit = 4096

var notFoundMessages = []string{
	"unknown revision",
	"no matching versions",
	"not a known dependency",
	"404 Not Found",
	"410 Gone",
}

// GoListFetcher resolves publish times via `go list -m -json mod@ver`,
// which uses the Go toolchain's own proxy client and so honors GOPROXY,
// GOPRIVATE, and GONOSUMCHECK natively.
type GoListFetcher struct {
	Dir string // directory to run `go` in ("" = current)
	// runner overrides command execution in tests. nil means real execution.
	runner func(ctx context.Context, dir string, args ...string) (stdout, stderr []byte, err error)
}

// PublishTime returns the publish time reported by the Go toolchain.
func (f *GoListFetcher) PublishTime(
	ctx context.Context,
	modulePath string,
	version string,
) (time.Time, error) {
	runner := f.runner
	if runner == nil {
		runner = runGo
	}

	moduleVersion := modulePath + "@" + version
	stdout, stderr, err := runner(
		ctx,
		f.Dir,
		"list",
		"-m",
		"-mod=mod",
		"-json",
		moduleVersion,
	)
	if err != nil {
		stderrText := string(stderr)
		for _, message := range notFoundMessages {
			if strings.Contains(stderrText, message) {
				return time.Time{}, fmt.Errorf("go list %s: %w", moduleVersion, ErrVersionNotFound)
			}
		}
		return time.Time{}, fmt.Errorf(
			"go list %s failed: %w: %s",
			moduleVersion,
			err,
			stderrTail(stderr),
		)
	}

	var result struct {
		Time string `json:"Time"`
	}
	if err := json.Unmarshal(stdout, &result); err != nil {
		return time.Time{}, fmt.Errorf("decode go list result for %s: %w", moduleVersion, err)
	}
	if result.Time == "" {
		return time.Time{}, fmt.Errorf("go list %s returned no publish time: %w", moduleVersion, ErrVersionNotFound)
	}
	published, err := time.Parse(time.RFC3339, result.Time)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse go list publish time for %s: %w", moduleVersion, err)
	}
	return published, nil
}

func runGo(
	ctx context.Context,
	dir string,
	args ...string,
) (stdout, stderr []byte, err error) {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	var stdoutBuffer, stderrBuffer bytes.Buffer
	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer
	err = cmd.Run()
	return stdoutBuffer.Bytes(), stderrBuffer.Bytes(), err
}

func stderrTail(stderr []byte) string {
	if len(stderr) > stderrTailLimit {
		stderr = stderr[len(stderr)-stderrTailLimit:]
	}
	return strings.TrimSpace(string(stderr))
}
