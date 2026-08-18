package dependencyage

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	now := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	dir := initTestRepo(t)

	tests := []struct {
		name     string
		fetch    TimeFetcher
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{
			name: "young introduced version violates",
			fetch: func(_ context.Context, m, _ string) (time.Time, error) {
				if m == "a.com/x" {
					return now.AddDate(0, 0, -3), nil
				}
				return now.AddDate(0, 0, -100), nil
			},
			wantCode: 1,
			wantOut:  "VIOLATION a.com/x@v1.1.0",
		},
		{
			name: "all old passes",
			fetch: func(_ context.Context, _, _ string) (time.Time, error) {
				return now.AddDate(0, 0, -100), nil
			},
			wantCode: 0,
			wantOut:  "Checked 2 introduced dependency version(s); 0 violation(s).",
		},
		{
			name: "lookup failure exits 2",
			fetch: func(_ context.Context, _, _ string) (time.Time, error) {
				return time.Time{}, errors.New("boom")
			},
			wantCode: 2,
			wantErr:  "ERROR could not verify dependency ages, failing closed:",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out, errW bytes.Buffer
			code := Run(context.Background(), Config{
				BaseRef:       "main",
				ThresholdDays: 14,
				Fetch:         tc.fetch,
				Source:        &GitSource{Dir: dir},
				Now:           now,
				Out:           &out,
				Err:           &errW,
			})
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr: %s)", code, tc.wantCode, errW.String())
			}
			if tc.wantOut != "" && !strings.Contains(out.String(), tc.wantOut) {
				t.Fatalf("stdout %q does not contain %q", out.String(), tc.wantOut)
			}
			if tc.wantErr != "" && !strings.Contains(errW.String(), tc.wantErr) {
				t.Fatalf("stderr %q does not contain %q", errW.String(), tc.wantErr)
			}
		})
	}
}
