// Copyright (c) 2021 Uber Technologies, Inc.
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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/constants"
)

func defaultFilestoreConfig(t *testing.T) *YamlNode {
	node, err := ToYamlNode(&FilestoreArchiver{
		FileMode: "044",
	})
	require.NoError(t, err)
	return node
}

// History archival

func TestValidEnabledHistoryArchivalConfig(t *testing.T) {
	archival := Archival{
		History: HistoryArchival{
			Status: constants.ArchivalEnabled,
			Provider: HistoryArchiverProvider{
				FilestoreConfig: defaultFilestoreConfig(t),
			},
		},
	}
	err := archival.Validate(&ArchivalDomainDefaults{
		History: HistoryArchivalDomainDefaults{
			URI: "/var/tmp",
		},
	})
	require.NoError(t, err)
}

func TestInvalidHEnabledHistoryArchivalConfig(t *testing.T) {
	archival := Archival{
		History: HistoryArchival{
			Status: constants.ArchivalEnabled,
		},
	}
	err := archival.Validate(&ArchivalDomainDefaults{})
	require.Error(t, err)
}

func TestValidDisabledHistoryArchivalConfig(t *testing.T) {
	archival := Archival{
		History: HistoryArchival{
			Provider: HistoryArchiverProvider{
				FilestoreConfig: defaultFilestoreConfig(t),
			},
		},
	}
	err := archival.Validate(&ArchivalDomainDefaults{})
	require.NoError(t, err)
}

func TestInvalidDisabledHistoryArchivalConfig(t *testing.T) {
	archival := Archival{
		History: HistoryArchival{
			EnableRead: true,
		},
	}
	err := archival.Validate(&ArchivalDomainDefaults{})
	require.Error(t, err)
}

func TestValidEmptyHistoryArchivalConfig(t *testing.T) {
	archival := Archival{
		History: HistoryArchival{},
	}
	err := archival.Validate(&ArchivalDomainDefaults{})
	require.NoError(t, err)
}

// Visibility archival

func TestValidEnabledVisibilityArchivalConfig(t *testing.T) {
	archival := Archival{
		Visibility: VisibilityArchival{
			Status: constants.ArchivalEnabled,
			Provider: VisibilityArchiverProvider{
				FilestoreConfig: defaultFilestoreConfig(t),
			},
		},
	}
	err := archival.Validate(&ArchivalDomainDefaults{
		Visibility: VisibilityArchivalDomainDefaults{
			URI: "/var/tmp",
		},
	})
	require.NoError(t, err)
}

func TestInvalidHEnabledVisibilityArchivalConfig(t *testing.T) {
	archival := Archival{
		Visibility: VisibilityArchival{
			Status: constants.ArchivalEnabled,
		},
	}
	err := archival.Validate(&ArchivalDomainDefaults{})
	require.Error(t, err)
}

func TestValidDisabledVisibilityArchivalConfig(t *testing.T) {
	archival := Archival{
		Visibility: VisibilityArchival{
			Provider: VisibilityArchiverProvider{
				FilestoreConfig: defaultFilestoreConfig(t),
			},
		},
	}
	err := archival.Validate(&ArchivalDomainDefaults{})
	require.NoError(t, err)
}

func TestInvalidDisabledVisibilityArchivalConfig(t *testing.T) {
	archival := Archival{
		Visibility: VisibilityArchival{
			EnableRead: true,
		},
	}
	err := archival.Validate(&ArchivalDomainDefaults{})
	require.Error(t, err)
}

func TestValidEmptyVisibilityArchivalConfig(t *testing.T) {
	archival := Archival{
		Visibility: VisibilityArchival{},
	}
	err := archival.Validate(&ArchivalDomainDefaults{})
	require.NoError(t, err)
}
