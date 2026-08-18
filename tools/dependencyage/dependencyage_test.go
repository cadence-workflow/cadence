package dependencyage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const goModBase = `module github.com/uber/cadence

go 1.23

require (
	github.com/stretchr/testify v1.8.0
	go.uber.org/zap v1.24.0 // indirect
)

require github.com/Shopify/sarama v1.38.1
`

const goModHead = `module github.com/uber/cadence

go 1.23

require (
	github.com/stretchr/testify v1.9.0
	go.uber.org/zap v1.24.0 // indirect
	github.com/new/dep v0.1.0
)

require github.com/Shopify/sarama v1.38.1
`

const goModReplaceBase = `module github.com/uber/cadence

go 1.23

require github.com/foo/bar v1.0.0

replace github.com/foo/bar => github.com/foo/bar v1.0.1
`

const goModReplaceHead = `module github.com/uber/cadence

go 1.23

require github.com/foo/bar v1.0.0

replace github.com/foo/bar => github.com/evil/fork v0.0.0-20260812000000-abcdef123456

replace (
	github.com/baz/qux/v2 v2.0.0 => github.com/baz/qux/v2 v2.1.0
	github.com/local/dep => ../localdep
)
`

const goModRepinBase = `module github.com/uber/cadence

go 1.23

require github.com/foo/bar v1.0.0

replace github.com/foo/bar v1.0.0 => github.com/fork/bar v5.0.0
`

const goModRepinHead = `module github.com/uber/cadence

go 1.23

require github.com/foo/bar v1.2.0

replace github.com/foo/bar v1.2.0 => github.com/fork/bar v5.0.0
`

func TestParseRequires(t *testing.T) {
	got, err := ParseRequires(goModBase)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"github.com/stretchr/testify": "v1.8.0",
		"go.uber.org/zap":             "v1.24.0",
		"github.com/Shopify/sarama":   "v1.38.1",
	}, got)
}

func TestParseRequiresInvalid(t *testing.T) {
	_, err := ParseRequires("module \x00 not a gomod ((")
	require.Error(t, err)
}

func TestNewRequirements(t *testing.T) {
	tests := []struct {
		name string
		base string
		head string
		want []ModuleVersion
	}{
		{
			name: "bumped and new modules reported, unchanged not",
			base: goModBase,
			head: goModHead,
			want: []ModuleVersion{
				{"github.com/new/dep", "v0.1.0"},
				{"github.com/stretchr/testify", "v1.9.0"},
			},
		},
		{
			name: "empty base reports everything",
			base: "",
			head: goModBase,
			want: []ModuleVersion{
				{"github.com/Shopify/sarama", "v1.38.1"},
				{"github.com/stretchr/testify", "v1.8.0"},
				{"go.uber.org/zap", "v1.24.0"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewRequirements(tc.base, tc.head)
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.want, got)
		})
	}
}

func TestNewReplacements(t *testing.T) {
	tests := []struct {
		name string
		base string
		head string
		want []ModuleVersion
	}{
		{
			name: "changed and added replacements reported, filesystem skipped",
			base: goModReplaceBase,
			head: goModReplaceHead,
			want: []ModuleVersion{
				{"github.com/baz/qux/v2", "v2.1.0"},
				{"github.com/evil/fork", "v0.0.0-20260812000000-abcdef123456"},
			},
		},
		{
			name: "unchanged replacement not reported",
			base: goModReplaceBase,
			head: goModReplaceBase,
			want: nil,
		},
		{
			name: "left-side version repin with identical target not reported",
			base: goModRepinBase,
			head: goModRepinHead,
			want: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewReplacements(tc.base, tc.head)
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.want, got)
		})
	}
}

func TestFindViolations(t *testing.T) {
	now := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	ctx := context.Background()

	t.Run("young version violates, old version passes", func(t *testing.T) {
		times := map[ModuleVersion]time.Time{
			{"a.com/young", "v1.0.0"}: now.AddDate(0, 0, -5),
			{"a.com/old", "v1.0.0"}:   now.AddDate(0, 0, -20),
		}
		fetch := func(_ context.Context, m, v string) (time.Time, error) {
			if t, ok := times[ModuleVersion{m, v}]; ok {
				return t, nil
			}
			return time.Time{}, ErrVersionNotFound
		}
		got, err := FindViolations(ctx,
			[]ModuleVersion{{"a.com/young", "v1.0.0"}, {"a.com/old", "v1.0.0"}},
			14, now, fetch, &bytes.Buffer{})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, ModuleVersion{"a.com/young", "v1.0.0"}, got[0].ModuleVersion)
	})

	t.Run("not-found with no pseudo-version warns but does not violate", func(t *testing.T) {
		var warn bytes.Buffer
		fetch := func(_ context.Context, m, v string) (time.Time, error) {
			return time.Time{}, fmt.Errorf("%s@%s: %w", m, v, ErrVersionNotFound)
		}
		got, err := FindViolations(ctx,
			[]ModuleVersion{{"a.com/unknown", "v1.0.0"}}, 14, now, fetch, &warn)
		require.NoError(t, err)
		assert.Empty(t, got)
		assert.Contains(t, warn.String(), "WARN")
	})

	t.Run("not-found with pseudo-version falls back to its timestamp", func(t *testing.T) {
		fetch := func(_ context.Context, _, _ string) (time.Time, error) {
			return time.Time{}, ErrVersionNotFound
		}
		got, err := FindViolations(ctx,
			[]ModuleVersion{{"a.com/pseudo", "v0.0.0-20260815000000-abcdef123456"}},
			14, now, fetch, &bytes.Buffer{})
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("non-not-found error fails closed", func(t *testing.T) {
		fetch := func(_ context.Context, _, _ string) (time.Time, error) {
			return time.Time{}, errors.New("network sadness")
		}
		_, err := FindViolations(ctx,
			[]ModuleVersion{{"a.com/x", "v1.0.0"}}, 14, now, fetch, &bytes.Buffer{})
		require.Error(t, err)
	})
}
