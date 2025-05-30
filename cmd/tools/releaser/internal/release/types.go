package release

import "github.com/Masterminds/semver/v3"

// State holds the current state of the repository
type State struct {
	CurrentBranch  string
	Modules        []Module
	CurrentVersion string
	TagCache       *TagCache
}

// Module represents a Go module
type Module struct {
	Path    string
	Version string
}

// ParsedTag represents a single parsed git tag
type ParsedTag struct {
	Raw           string          // Original tag name (e.g., "service1/v1.2.3-prerelease01")
	ModulePath    string          // Module path (e.g., "service1", "" for root)
	Version       *semver.Version // Parsed semantic version
	GitSHA        string          // Git commit SHA
	IsPrerelease  bool
	PrereleaseNum int // Extracted prerelease number (01, 02, etc.)
}

// TagCache holds all parsed tag information
type TagCache struct {
	AllTags         []ParsedTag
	VersionTags     []ParsedTag            // Only valid semver tags
	ModuleTags      map[string][]ParsedTag // Tags grouped by module path
	PrereleaseCache map[string][]int       // Base version -> prerelease numbers
	HighestVersion  *semver.Version        // Cached highest version
}

// WarningType represents different types of warnings
type WarningType int

const (
	WrongBranch WarningType = iota
	ExistingTags
)

// Warning represents a validation warning that can be overridden
type Warning struct {
	Type    WarningType
	Message string
}

// Action represents a planned release action
type Action struct {
	Type        ActionType
	Target      string // tag name, module path
	Description string
	GitSHA      string // for existing tags
}

type ActionType int

const (
	ActionCreateTag ActionType = iota
	ActionPushTags
)
