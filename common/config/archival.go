// Copyright (c) 2019 Uber Technologies, Inc.
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

package config

import (
	"errors"

	"github.com/uber/cadence/common/constants"
)

// Validate validates the archival config
func (a *Archival) Validate(domainDefaults *ArchivalDomainDefaults) error {
	if a.History.Status == constants.ArchivalEnabled {
		if domainDefaults.History.URI == "" || a.History.Provider == nil {
			return errors.New("invalid history archival config, must provide domainDefaults.History.URI and Provider")
		}
	} else {
		if a.History.EnableRead {
			return errors.New("invalid history archival config, cannot EnableRead when archival is disabled")
		}
	}

	if a.Visibility.Status == constants.ArchivalEnabled {
		if domainDefaults.Visibility.URI == "" || a.Visibility.Provider == nil {
			return errors.New("invalid visibility archival config, must provide domainDefaults.Visibility.URI and Provider")
		}
	} else {
		if a.Visibility.EnableRead {
			return errors.New("invalid visibility archival config, cannot EnableRead when archival is disabled")
		}
	}

	return nil
}
