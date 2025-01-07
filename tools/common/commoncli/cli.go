// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package commoncli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
)

var (
	colorRed     = color.New(color.FgRed).SprintFunc()
	colorMagenta = color.New(color.FgMagenta).SprintFunc()
)

// ExitHandler converts errors that urfave/cli did not handle into a nicely
// printed message, and an appropriate os.Exit call to ensure this func never
// returns from either branch.
//
// This should be used instead of an in-CLI ExitErrHandler, as using that still
// leaves a dangling `err` return value from *cli.App.Run(), which is awkward
// in a number of ways.
func ExitHandler(err error) {
	if err == nil {
		// no error, which (oddly) includes some command-not-found paths
		os.Exit(0)
	}

	// print what we can to stderr.
	// and ignore errs while doing so, since we have no real alternative.
	_ = printErr(err, os.Stderr)

	// all errors are "fatal", unlike default behavior which only fails if you
	// return an ExitCoder error.
	os.Exit(1)
}

// prints this (possibly printable) error to the given io.Writer.
// write-errors will be returned, if any are encountered.
func printErr(err error, to io.Writer) (writeErr error) {
	// the way Go does error wrapping is really a massive pain, as the stdlib encourages
	// people to wrap *and duplicate* basically every error message, because they don't
	// provide any API for getting "just this error" content.
	//
	// we can live with that.
	// build strings recursively, stripping out matching suffixes, and hope for the best.
	// it's quite wasteful, but it's only done once per process so it's fine.

	// error-coalescing write-helper.
	// if an error is encountered, the first will be saved, and later calls will no-op.
	write := func(format string, a ...any) {
		if writeErr != nil {
			return
		}
		_, tmpErr := fmt.Fprintf(to, format, a...)
		if tmpErr != nil {
			writeErr = tmpErr
		}
	}

	var topPrintable *printableErr
	_ = errors.As(err, &topPrintable)

	var unwrapped []error
	unwrapped = append(unwrapped, err)
	current := err
	for i := 0; i < 1000; i++ { // 1k would be an insane depth, likely a loop
		next := errors.Unwrap(current)
		if next != nil {
			current = next
			unwrapped = append(unwrapped, next)
		} else {
			break
		}
	}

	type parsed struct {
		err error
		msg string
	}
	allParsed := make([]parsed, len(unwrapped))
	for i := range unwrapped {
		p := parsed{err: unwrapped[i]}
		if perr, ok := unwrapped[i].(*printableErr); ok {
			p.msg = perr.display
			if i > 0 && perr == topPrintable { // must be same instance somewhere deeper in the stack of errors
				p.msg = "Error (above): " + p.msg
			} else {
				p.msg = "Error: " + p.msg
			}
		} else {
			p.msg = unwrapped[i].Error()
		}
		if i > 0 {
			// trim current from previous (needs to be err.Error() instead of p.msg or the special "Error: " prefixing above will break trimming),
			// and attempt to trim off any trailing ": " because it's so common.
			// this is not an exact science, but neither are error messages so it's just a hopeful hack.
			allParsed[i-1].msg = strings.TrimSuffix(
				strings.TrimSpace(
					strings.TrimSuffix(
						strings.TrimSpace(allParsed[i-1].msg),
						strings.TrimSpace(p.err.Error()),
					),
				),
				":",
			)
		}
		allParsed[i] = p
	}

	if topPrintable != nil {
		write("%s %s\n", colorRed("Error:"), topPrintable.display)
		if allParsed[0].err == topPrintable {
			allParsed = allParsed[1:] // already printed, skip
		}
	} else { // len(allParsed) > 0
		write("%s %s\n", colorRed("Error:"), allParsed[0].msg)
		allParsed = allParsed[1:] // already printed, skip
	}

	if len(allParsed) == 0 {
		return // no details to print
	}

	indent := "  "
	write("%s\n", colorMagenta("Error details:")) // only the top level gets color
	// and now write the rest
	needDetails := false
	for _, this := range allParsed {
		lines := strings.Split(this.msg, "\n")
		if needDetails {
			write("%s%s\n", indent, "Error details:")
			indent += "  " // next errors indent further
			needDetails = false
		}
		for _, line := range lines {
			write("%s%s\n", indent, line)
		}
		if _, ok := this.err.(*printableErr); ok { // already unwrapped
			needDetails = true
		}
	}
	return
}

// Problem returns a typed error that will report this message "nicely" to the
// user if it exits the CLI app.  The message will be used as the top-level
// "Error: ..." string regardless of where in the error stack it is, and other
// wrapped errors will be printed line by line beneath it.
//
// Nested Problem messages will nest structurally, like:
//
//	Error: msg
//	Details:
//	  some error
//	  some other error
//	  Error: message
//	  ErrorDetails:
//	    more nested errors
func Problem(msg string, err error) error {
	return &printableErr{msg, err}
}

type printableErr struct {
	display string
	cause   error
}

func (p *printableErr) Error() string {
	buf := strings.Builder{}
	buf.WriteString(p.display)
	if p.cause != nil {
		buf.WriteString(": ")
		buf.WriteString(p.cause.Error())
	}
	return buf.String()
}

func (p *printableErr) Unwrap() error {
	return p.cause
}
