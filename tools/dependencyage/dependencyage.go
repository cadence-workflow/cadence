// Package dependencyage contains the core of the dependency-age CI gate. It is
// deliberately free of git, HTTP, and CLI concerns so it can later be extracted
// into a standalone GitHub Action.
package dependencyage

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// ModuleVersion is one introduced module path and version pair.
type ModuleVersion struct {
	Module  string
	Version string
}

// Violation is an introduced version younger than the configured threshold.
type Violation struct {
	ModuleVersion
	Published time.Time
}

// ErrVersionNotFound reports that the module source affirmatively does not
// know the version (as opposed to a lookup failure, which must fail closed).
var ErrVersionNotFound = errors.New("module version not found")

// TimeFetcher returns the publish time of a module version.
// Returns ErrVersionNotFound (wrapped or bare; matched via errors.Is) when
// the version is unknown; any other error fails the check closed.
type TimeFetcher func(ctx context.Context, module, version string) (time.Time, error)

// Replacement is one replace directive. A nil New target represents a
// filesystem-path replacement.
type Replacement struct {
	Old ModuleVersion
	New *ModuleVersion
}

// ParseRequires returns the module path and version from every require
// directive in go.mod content.
func ParseRequires(content string) (map[string]string, error) {
	file, err := parseModFile(content)
	if err != nil {
		return nil, err
	}

	requires := make(map[string]string)
	for _, require := range file.Require {
		requires[require.Mod.Path] = require.Mod.Version
	}
	return requires, nil
}

// ParseReplaces returns every replace directive. A nil target represents a
// filesystem-path replacement.
func ParseReplaces(content string) ([]Replacement, error) {
	file, err := parseModFile(content)
	if err != nil {
		return nil, err
	}

	replaces := make([]Replacement, 0, len(file.Replace))
	for _, replace := range file.Replace {
		replacement := Replacement{
			Old: ModuleVersion{Module: replace.Old.Path, Version: replace.Old.Version},
		}
		if replace.New.Version != "" {
			replacement.New = &ModuleVersion{
				Module:  replace.New.Path,
				Version: replace.New.Version,
			}
		}
		replaces = append(replaces, replacement)
	}
	return replaces, nil
}

// NewRequirements returns head requirements that are new or have a different
// version than the corresponding base requirement.
func NewRequirements(baseContent, headContent string) ([]ModuleVersion, error) {
	base, err := ParseRequires(baseContent)
	if err != nil {
		return nil, err
	}
	head, err := ParseRequires(headContent)
	if err != nil {
		return nil, err
	}

	introduced := make([]ModuleVersion, 0, len(head))
	for modulePath, version := range head {
		if base[modulePath] != version {
			introduced = append(introduced, ModuleVersion{Module: modulePath, Version: version})
		}
	}
	sortModuleVersions(introduced)
	return introduced, nil
}

// NewReplacements returns versioned head replacement targets not present among
// the base replacement targets.
func NewReplacements(baseContent, headContent string) ([]ModuleVersion, error) {
	base, err := ParseReplaces(baseContent)
	if err != nil {
		return nil, err
	}
	head, err := ParseReplaces(headContent)
	if err != nil {
		return nil, err
	}

	baseTargets := make(map[ModuleVersion]struct{}, len(base))
	for _, replacement := range base {
		if replacement.New != nil {
			baseTargets[*replacement.New] = struct{}{}
		}
	}
	headTargets := make(map[ModuleVersion]struct{}, len(head))
	var introduced []ModuleVersion
	for _, replacement := range head {
		if replacement.New == nil {
			continue
		}
		target := *replacement.New
		if _, exists := baseTargets[target]; exists {
			continue
		}
		if _, exists := headTargets[target]; exists {
			continue
		}
		headTargets[target] = struct{}{}
		introduced = append(introduced, target)
	}
	sortModuleVersions(introduced)
	return introduced, nil
}

// FindViolations returns introduced module versions younger than thresholdDays.
// Unknown publish times fall back to pseudo-version timestamps. If no timestamp
// is available, or if fetching fails, the check aborts.
func FindViolations(
	ctx context.Context,
	pairs []ModuleVersion,
	thresholdDays int,
	now time.Time,
	fetch TimeFetcher,
) ([]Violation, error) {
	sortedPairs := append([]ModuleVersion(nil), pairs...)
	sortModuleVersions(sortedPairs)
	thresholdDuration := time.Duration(thresholdDays) * 24 * time.Hour

	var violations []Violation
	for _, pair := range sortedPairs {
		published, err := fetch(ctx, pair.Module, pair.Version)
		if errors.Is(err, ErrVersionNotFound) {
			var pseudoErr error
			published, pseudoErr = module.PseudoVersionTime(pair.Version)
			if pseudoErr != nil {
				return nil, fmt.Errorf(
					"determine publish time for %s@%s: %w; pseudo-version fallback: %v",
					pair.Module,
					pair.Version,
					err,
					pseudoErr,
				)
			}
		} else if err != nil {
			return nil, fmt.Errorf("fetch publish time for %s@%s: %w", pair.Module, pair.Version, err)
		}

		if now.Sub(published) < thresholdDuration {
			violations = append(violations, Violation{
				ModuleVersion: pair,
				Published:     published,
			})
		}
	}

	return violations, nil
}

func parseModFile(content string) (*modfile.File, error) {
	return modfile.Parse("go.mod", []byte(content), nil)
}

func sortModuleVersions(pairs []ModuleVersion) {
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Module == pairs[j].Module {
			return pairs[i].Version < pairs[j].Version
		}
		return pairs[i].Module < pairs[j].Module
	})
}
