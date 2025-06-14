package console

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfirm(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
		hasError bool
	}{
		{"yes response", "y\n", true, false},
		{"Yes response", "Y\n", true, false},
		{"yes full", "yes\n", true, false},
		{"no response", "n\n", false, false},
		{"No response", "N\n", false, false},
		{"no full", "no\n", false, false},
		{"empty input uses default false", "\n", false, false},
		{"invalid input", "maybe\n", false, true},
		{"whitespace input", "  \n", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			writer := &bytes.Buffer{}
			manager := NewManager(reader, writer)

			ctx := context.Background()
			result, err := manager.Confirm(ctx, "Test message")

			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}

			// Check that prompt was written
			output := writer.String()
			assert.Contains(t, output, "Test message [y/N]:")
		})
	}
}

func TestConfirmWithDefault(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue bool
		expected     bool
		hasError     bool
	}{
		{"yes with default false", "y\n", false, true, false},
		{"no with default true", "n\n", true, false, false},
		{"empty uses default true", "\n", true, true, false},
		{"empty uses default false", "\n", false, false, false},
		{"invalid input", "invalid\n", true, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			writer := &bytes.Buffer{}
			manager := NewManager(reader, writer)

			ctx := context.Background()
			result, err := manager.ConfirmWithDefault(ctx, "Test message", tt.defaultValue)

			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}

			// Check prompt format based on default
			output := writer.String()
			if tt.defaultValue {
				assert.Contains(t, output, "[Y/n]:")
			} else {
				assert.Contains(t, output, "[y/N]:")
			}
		})
	}
}

// blockingReader blocks on Read until unblocked
type blockingReader struct {
	unblock chan struct{}
	data    []byte
	read    bool
}

func newBlockingReader() *blockingReader {
	return &blockingReader{
		unblock: make(chan struct{}),
		data:    []byte("y\n"),
	}
}

func (br *blockingReader) Read(p []byte) (n int, err error) {
	if br.read {
		return 0, io.EOF
	}

	// Block until unblocked
	<-br.unblock

	br.read = true
	n = copy(p, br.data)
	return n, nil
}

func (br *blockingReader) Unblock() {
	close(br.unblock)
}

func TestContextCancellation(t *testing.T) {
	t.Run("context cancelled before input", func(t *testing.T) {
		// Create a context that's already cancelled
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		reader := strings.NewReader("y\n") // This won't be read due to cancellation
		writer := &bytes.Buffer{}
		manager := NewManager(reader, writer)

		result, err := manager.ConfirmWithDefault(ctx, "Test", false)

		require.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))
		assert.False(t, result)
	})

	t.Run("context cancelled during input wait", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		// Use a blocking reader that actually blocks
		reader := newBlockingReader()
		writer := &bytes.Buffer{}
		manager := NewManager(reader, writer)

		// Cancel context after a short delay
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		result, err := manager.ConfirmWithDefault(ctx, "Test", false)

		require.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))
		assert.False(t, result)
	})
}

// errorReader always returns an error when scanning
type errorReader struct{}

func (e errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

func TestScannerError(t *testing.T) {
	reader := errorReader{}
	writer := &bytes.Buffer{}
	manager := NewManager(reader, writer)

	ctx := context.Background()
	result, err := manager.ConfirmWithDefault(ctx, "Test", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read input")
	assert.False(t, result)
}

func TestEOFHandling(t *testing.T) {
	// Reader with no content (immediate EOF)
	reader := strings.NewReader("")
	writer := &bytes.Buffer{}
	manager := NewManager(reader, writer)

	ctx := context.Background()
	result, err := manager.ConfirmWithDefault(ctx, "Test", true)

	assert.NoError(t, err)
	assert.True(t, result, "Expected default value (true) on EOF")
}
