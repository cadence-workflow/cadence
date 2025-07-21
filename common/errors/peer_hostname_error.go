// Copyright (c) 2018 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package errors

import (
	"errors"
	"fmt"
)

// PeerHostnameError wraps an error with peer hostname information
type PeerHostnameError struct {
	PeerHostname string
	WrappedError error
}

// Error implements the error interface
func (e *PeerHostnameError) Error() string {
	return fmt.Sprintf("peer hostname: %s, error: %v", e.PeerHostname, e.WrappedError)
}

// Unwrap implements the error unwrapping interface
func (e *PeerHostnameError) Unwrap() error {
	return e.WrappedError
}

// NewPeerHostnameError creates a new PeerHostnameError
func NewPeerHostnameError(err error, peer string) error {
	if err == nil {
		return nil
	}
	if peer == "" {
		return err
	}
	return &PeerHostnameError{
		PeerHostname: peer,
		WrappedError: err,
	}
}

// ExtractPeerHostname extracts the peer hostname from a wrapped error
// Returns the hostname and the original unwrapped error
func ExtractPeerHostname(err error) (string, error) {
	if err == nil {
		return "", nil
	}
	var peerErr *PeerHostnameError
	current := err
	if errors.As(current, &peerErr) {
		return peerErr.PeerHostname, peerErr.WrappedError
	}
	return "", err
}
