package scripting

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
)

// Executor is a helper for running scripting
type Executor interface {
	BashExec(ctx context.Context, in string) (stdout string, stderr string, exitErr *exec.ExitError)
	Exec(ctx context.Context, bin string, args ...string) (stdout string, stderr string, exitErr *exec.ExitError)
	QuietBashExec(ctx context.Context, in string) (stdout string, stderr string, exitErr *exec.ExitError)
	QuietExec(ctx context.Context, bin string, args ...string) (stdout string, stderr string, exitErr *exec.ExitError)
}

// New returns a new bash executor
func New() Executor {
	return &execImpl{}
}

type execImpl struct{}

// BashExec is a helper function for ensuring the user
// can read a legible streaming from a subprocess,
// as well as allowing programmatic access to stdout, stderr and exit codes
// it's highly unsafe for any kind of untrusted inputs as it's explicitly bypassing
// go's exec safety args, so it *must not* come into contact with anything untrusted
func (execImpl) BashExec(ctx context.Context, in string) (stdout string, stderr string, exitErr *exec.ExitError) {
	cmd := exec.CommandContext(ctx, "bash", "-c", in)
	var stdBuffer bytes.Buffer
	var stdErrBuffer bytes.Buffer
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	mwErr := io.MultiWriter(os.Stderr, &stdErrBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mwErr
	err := cmd.Run()
	var e *exec.ExitError
	if errors.As(err, &e) && e.ExitCode() != 0 {
		return stdBuffer.String(), stdErrBuffer.String(), e
	} else if err != nil {
		panic(err)
	}
	return stdBuffer.String(), stdErrBuffer.String(), nil
}

// Exec is a wrapper around exec.Command which adds some convenience
// functionality to both capture standout/err as well as tee it to the user's UI in real time
// meaning that the user doesn't need to wait for the command to complete.
// It's value is fairly marginal and if it presents any problems the user should consider just
// using exec.Command directly
func (execImpl) Exec(ctx context.Context, bin string, args ...string) (stdout string, stderr string, exitErr *exec.ExitError) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdBuffer bytes.Buffer
	var stdErrBuffer bytes.Buffer
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	mwErr := io.MultiWriter(os.Stderr, &stdErrBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mwErr
	err := cmd.Run()
	var e *exec.ExitError
	if errors.As(err, &e) && e.ExitCode() != 0 {
		return stdBuffer.String(), stdErrBuffer.String(), e
	} else if err != nil {
		panic(err)
	}
	return stdBuffer.String(), stdErrBuffer.String(), nil
}

// QuietBashExec ...
func (execImpl) QuietBashExec(ctx context.Context, in string) (stdout string, stderr string, exitErr *exec.ExitError) {
	cmd := exec.CommandContext(ctx, "bash", "-c", in)
	var stdBuffer bytes.Buffer
	var stdErrBuffer bytes.Buffer
	cmd.Stdout = &stdBuffer
	cmd.Stderr = &stdErrBuffer
	err := cmd.Run()
	var e *exec.ExitError
	if errors.As(err, &e) && e.ExitCode() != 0 {
		return stdBuffer.String(), stdErrBuffer.String(), e
	} else if err != nil {
		panic(err)
	}
	return stdBuffer.String(), stdErrBuffer.String(), nil
}

// QuietExec ...
func (execImpl) QuietExec(ctx context.Context, bin string, args ...string) (stdout string, stderr string, exitErr *exec.ExitError) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdBuffer bytes.Buffer
	var stdErrBuffer bytes.Buffer
	cmd.Stdout = &stdBuffer
	cmd.Stderr = &stdErrBuffer
	err := cmd.Run()
	var e *exec.ExitError
	if errors.As(err, &e) && e.ExitCode() != 0 {
		return stdBuffer.String(), stdErrBuffer.String(), e
	} else if err != nil {
		panic(err)
	}
	return stdBuffer.String(), stdErrBuffer.String(), nil
}
