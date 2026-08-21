package dependencyage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeRunner(stdout, stderr string, err error) func(context.Context, string, ...string) ([]byte, []byte, error) {
	return func(_ context.Context, _ string, _ ...string) ([]byte, []byte, error) {
		return []byte(stdout), []byte(stderr), err
	}
}

func TestGoListFetcherPublishTime(t *testing.T) {
	ctx := context.Background()
	execErr := errors.New("exit status 1")

	tests := []struct {
		name     string
		stdout   string
		stderr   string
		execErr  error
		want     time.Time
		notFound bool
		failed   bool
	}{
		{
			name:   "success parses Time",
			stdout: `{"Path":"github.com/stretchr/testify","Version":"v1.10.0","Time":"2024-11-12T22:58:45Z"}`,
			want:   time.Date(2024, 11, 12, 22, 58, 45, 0, time.UTC),
		},
		{
			name:     "missing Time is not found",
			stdout:   `{"Path":"a.com/x","Version":"v1.0.0"}`,
			notFound: true,
		},
		{
			name:     "unknown revision is not found",
			stderr:   `go: a.com/x@v1.0.0: invalid version: unknown revision v1.0.0`,
			execErr:  execErr,
			notFound: true,
		},
		{
			name:     "no matching versions is not found",
			stderr:   `go: a.com/x@v9: no matching versions for query "v9"`,
			execErr:  execErr,
			notFound: true,
		},
		{
			name:    "unclassified failure fails closed",
			stderr:  `go: dial tcp: lookup proxy.golang.org: no such host`,
			execErr: execErr,
			failed:  true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &GoListFetcher{runner: fakeRunner(tc.stdout, tc.stderr, tc.execErr)}
			got, err := f.PublishTime(ctx, "a.com/x", "v1.0.0")
			switch {
			case tc.notFound:
				require.ErrorIs(t, err, ErrVersionNotFound)
			case tc.failed:
				require.Error(t, err)
				require.False(t, errors.Is(err, ErrVersionNotFound))
			default:
				require.NoError(t, err)
				assert.Equal(t, tc.want, got.UTC())
			}
		})
	}
}

func TestGoListFetcherRealToolchain(t *testing.T) {
	if testing.Short() {
		t.Skip("network")
	}
	f := &GoListFetcher{}
	got, err := f.PublishTime(context.Background(), "github.com/stretchr/testify", "v1.10.0")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 11, 12, 22, 58, 45, 0, time.UTC), got.UTC())
}
