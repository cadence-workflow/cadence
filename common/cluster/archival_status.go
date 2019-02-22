// Copyright (c) 2017 Uber Technologies, Inc.
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

package cluster

type (
	// ArchivalStatus represents the archival status of the cluster
	ArchivalStatus int

	// ArchivalConfig is an immutable representation of the current cluster configuration of archival
	ArchivalConfig struct {
		status        ArchivalStatus
		defaultBucket string
	}
)

const (
	// ArchivalDisabled means this cluster is not configured to handle archival
	ArchivalDisabled ArchivalStatus = iota
	// ArchivalPaused means this cluster is configured to handle archival but is currently not archiving
	ArchivalPaused
	// ArchivalEnabled means this cluster is currently archiving
	ArchivalEnabled
)

// GetArchivalStatus converts input string to ArchivalStatus.
// Valid input strings are "disabled", "paused", and "enabled".
// If non-valid string is given status of Disabled is returned.
func GetArchivalStatus(str string) ArchivalStatus {
	switch str {
	case "disabled":
		return ArchivalDisabled
	case "paused":
		return ArchivalPaused
	case "enabled":
		return ArchivalEnabled
	default:
		return ArchivalDisabled
	}
}

// IsValid returns true if ArchivalConfig is valid, false otherwise.
func (a *ArchivalConfig) IsValid() bool {
	bucketSet := len(a.defaultBucket) != 0
	disabled := a.status == ArchivalDisabled
	return (!bucketSet && disabled) || (bucketSet && !disabled)
}

// GetDefaultBucket returns the default bucket for ArchivalConfig
func (a *ArchivalConfig) GetDefaultBucket() string {
	return a.defaultBucket
}

// GetArchivalStatus returns the archival status for ArchivalConfig
func (a *ArchivalConfig) GetArchivalStatus() ArchivalStatus {
	return a.status
}
