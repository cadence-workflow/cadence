package console

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// Manager handles console interactions
type Manager struct {
	reader io.Reader
	writer io.Writer
}

// NewManager creates a new console manager
func NewManager(reader io.Reader, writer io.Writer) *Manager {
	return &Manager{
		reader: reader,
		writer: writer,
	}
}

// Confirm asks for user confirmation and returns true for 'y', false for 'n'
func (m *Manager) Confirm(ctx context.Context, message string) (bool, error) {
	return m.ConfirmWithDefault(ctx, message, false)
}

// ConfirmWithDefault asks for user confirmation with a default value
// Returns defaultValue if user just presses enter
func (m *Manager) ConfirmWithDefault(ctx context.Context, message string, defaultValue bool) (bool, error) {
	prompt := fmt.Sprintf("%s [y/N]: ", message)
	if defaultValue {
		prompt = fmt.Sprintf("%s [Y/n]: ", message)
	}

	_, _ = fmt.Fprint(m.writer, prompt)

	inputChan := make(chan inputResult, 1)

	// Start goroutine to read input
	go func() {
		scanner := bufio.NewScanner(m.reader)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				inputChan <- inputResult{err: fmt.Errorf("failed to read input: %w", err)}
				return
			}
			// EOF or no input - use default
			inputChan <- inputResult{text: "", err: nil}
			return
		}
		inputChan <- inputResult{text: scanner.Text(), err: nil}
	}()

	var result inputResult
	// Wait for either input or context cancellation
	select {
	case <-ctx.Done():
		// Context was cancelled
		return false, ctx.Err()
	case result = <-inputChan:
	}
	// Got input from user
	if result.err != nil {
		return false, result.err
	}

	input := strings.TrimSpace(strings.ToLower(result.text))

	// Empty input uses default
	if input == "" {
		return defaultValue, nil
	}

	switch input {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid input: %s", input)
	}
}

type inputResult struct {
	text string
	err  error
}
