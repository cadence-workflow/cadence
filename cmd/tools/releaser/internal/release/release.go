package release

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination release_mocks_test.go -self_package github.com/uber/cadence/cmd/tools/releaser/release

// Git defines git operations for testing
type Git interface {
	GetCurrentBranch(ctx context.Context) (string, error)
	GetTags(ctx context.Context) ([]string, error)
	CreateTag(ctx context.Context, tag string) error
	PushTag(ctx context.Context, tag string) error
	GetRepoRoot(ctx context.Context) (string, error)
}

// FS defines filesystem operations for testing
type FS interface {
	FindGoModFiles(ctx context.Context, root string) ([]string, error)
}

// UserInteraction defines the interface for user interactions
type UserInteraction interface {
	Confirm(ctx context.Context, message string) (bool, error)
	ConfirmWithDefault(ctx context.Context, message string, defaultValue bool) (bool, error)
}

var (
	versionRegex    = regexp.MustCompile(`v[0-9]+\.[0-9]+\.[0-9]+(?:-prerelease[0-9]+)?`)
	prereleaseRegex = regexp.MustCompile(`v([0-9]+\.[0-9]+\.[0-9]+)-prerelease(\d+)`)
)

// Config holds the release configuration
type Config struct {
	RepoRoot       string
	Prerelease     bool
	Version        string
	VersionType    string
	ExcludedDirs   []string
	RequiredBranch string
	Verbose        bool

	// Interactive mode settings
	SkipConfirmations bool
}

// Manager handles the release process
type Manager struct {
	config      *Config
	git         Git
	fs          FS
	interaction UserInteraction
	tagCache    *TagCache
}

func NewReleaseManager(config *Config, git Git, fs FS, interaction UserInteraction) *Manager {
	return &Manager{
		config:      config,
		git:         git,
		fs:          fs,
		interaction: interaction,
	}
}

// Run runs the manager flow.
func (rm *Manager) Run(ctx context.Context) error {
	fmt.Printf("Starting release process with arguments\nversion %s, type %s, prerelease %T\n", rm.config.Version, rm.config.VersionType, rm.config.Prerelease)

	// Get known releases ONCE at the start and use it for all subsequent operations
	if err := rm.GetKnownReleases(ctx); err != nil {
		return err
	}

	// Two-mode operation: if no version specified, show current state
	if rm.config.Version == "" && rm.config.VersionType == "" && !rm.config.Prerelease {
		return rm.ShowCurrentState(ctx)
	}

	// Calculate target version first
	currentVersion := rm.GetCurrentGlobalVersion()
	targetVersion, err := rm.calculateNewVersion(currentVersion)
	if err != nil {
		return err
	}
	return rm.InteractiveRelease(ctx, targetVersion)
}

// GetKnownReleases fetches and parses all tags once
func (rm *Manager) GetKnownReleases(ctx context.Context) error {
	fmt.Println("Getting known releases")

	// Fetch raw tags once
	rawTags, err := rm.git.GetTags(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch tags: %w", err)
	}

	rm.tagCache = &TagCache{
		AllTags:         make([]ParsedTag, 0, len(rawTags)),
		VersionTags:     make([]ParsedTag, 0, len(rawTags)),
		ModuleTags:      make(map[string][]ParsedTag),
		PrereleaseCache: make(map[string][]int),
	}

	for _, rawTag := range rawTags {
		parsedTag := rm.parseTag(rawTag)
		rm.tagCache.AllTags = append(rm.tagCache.AllTags, parsedTag)

		// Skip not version tags
		if parsedTag.Version == nil {
			continue
		}

		rm.tagCache.VersionTags = append(rm.tagCache.VersionTags, parsedTag)

		// Group by module
		if rm.tagCache.ModuleTags[parsedTag.ModulePath] == nil {
			rm.tagCache.ModuleTags[parsedTag.ModulePath] = make([]ParsedTag, 0)
		}
		rm.tagCache.ModuleTags[parsedTag.ModulePath] = append(
			rm.tagCache.ModuleTags[parsedTag.ModulePath], parsedTag)

		// Cache prerelease numbers
		if parsedTag.IsPrerelease {
			baseVersion := fmt.Sprintf("v%d.%d.%d",
				parsedTag.Version.Major(),
				parsedTag.Version.Minor(),
				parsedTag.Version.Patch())

			if rm.tagCache.PrereleaseCache[baseVersion] == nil {
				rm.tagCache.PrereleaseCache[baseVersion] = make([]int, 0)
			}
			rm.tagCache.PrereleaseCache[baseVersion] = append(
				rm.tagCache.PrereleaseCache[baseVersion], parsedTag.PrereleaseNum)
		}
	}

	// Sort and cache highest version
	if len(rm.tagCache.VersionTags) > 0 {
		rm.sortVersionTags()
		rm.tagCache.HighestVersion = rm.tagCache.VersionTags[len(rm.tagCache.VersionTags)-1].Version
	}

	rm.logDebug("Known releases total tags (%d), version_tags(%d)", len(rm.tagCache.AllTags), len(rm.tagCache.VersionTags))

	return nil
}

func (rm *Manager) parseTag(rawTag string) ParsedTag {
	parsed := ParsedTag{Raw: rawTag}

	// Extract module path and version part
	if idx := strings.LastIndex(rawTag, "/v"); idx != -1 {
		parsed.ModulePath = rawTag[:idx]
		versionPart := rawTag[idx+1:]

		if versionRegex.MatchString(versionPart) {
			if version, err := semver.NewVersion(versionPart); err == nil {
				parsed.Version = version

				// Check for prerelease
				if matches := prereleaseRegex.FindStringSubmatch(versionPart); len(matches) > 2 {
					parsed.IsPrerelease = true
					if num, err := strconv.Atoi(matches[2]); err == nil {
						parsed.PrereleaseNum = num
					}
				}
			}
		}
		return parsed
	}

	// Root module tag
	if versionRegex.MatchString(rawTag) {
		if version, err := semver.NewVersion(rawTag); err == nil {
			parsed.Version = version

			if matches := prereleaseRegex.FindStringSubmatch(rawTag); len(matches) > 2 {
				parsed.IsPrerelease = true
				if num, err := strconv.Atoi(matches[2]); err == nil {
					parsed.PrereleaseNum = num
				}
			}
		}
	}

	return parsed
}

func (rm *Manager) sortVersionTags() {
	sort.Slice(rm.tagCache.VersionTags, func(i, j int) bool {
		return rm.tagCache.VersionTags[i].Version.LessThan(rm.tagCache.VersionTags[j].Version)
	})
}

func (rm *Manager) GetCurrentGlobalVersion() string {
	if rm.tagCache.HighestVersion == nil {
		return "v0.0.0"
	}

	return "v" + rm.tagCache.HighestVersion.String()
}

func (rm *Manager) GetNextPrereleaseVersion(baseVersionStr string) (string, error) {
	// Parse base version
	baseVersion, err := semver.NewVersion(baseVersionStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse base version %s: %w", baseVersionStr, err)
	}

	cleanBaseStr := fmt.Sprintf("v%d.%d.%d", baseVersion.Major(), baseVersion.Minor(), baseVersion.Patch())

	prereleaseNumbers := rm.tagCache.PrereleaseCache[cleanBaseStr]
	if len(prereleaseNumbers) == 0 {
		return fmt.Sprintf("%s-prerelease01", cleanBaseStr), nil
	}

	// Sort and get next number
	sort.Ints(prereleaseNumbers)
	nextNum := prereleaseNumbers[len(prereleaseNumbers)-1] + 1

	// Check for single-digit format
	if nextNum < 10 {
		return "", fmt.Errorf("latest prerelease uses 1-digit format. Only 2-digit format supported, base (%s), number (%d)", cleanBaseStr, nextNum)
	}

	if nextNum > 99 {
		return "", fmt.Errorf("maximum prerelease number (99) exceeded, base (%s)", cleanBaseStr)
	}

	return fmt.Sprintf("%s-prerelease%02d", cleanBaseStr, nextNum), nil
}

func (rm *Manager) CheckVersionExists(version string, modules []Module) error {
	var existingTags []string

	for _, module := range modules {
		expectedTag := version
		if module.Path != "" {
			expectedTag = module.Path + "/" + version
		}

		for _, tag := range rm.tagCache.AllTags {
			if tag.Raw == expectedTag {
				existingTags = append(existingTags, expectedTag)
				break
			}
		}
	}

	if len(existingTags) > 0 {
		return fmt.Errorf("version already exists for modules: %v", existingTags)
	}

	return nil
}

// AssessCurrentState gathers repository state (assumes cache is already populated)
func (rm *Manager) AssessCurrentState(ctx context.Context) (*State, error) {
	state := &State{}

	var err error
	// Gather information (cache should already be populated)
	state.CurrentBranch, err = rm.git.GetCurrentBranch(ctx)
	if err != nil {
		return nil, fmt.Errorf("get current branch: %w", err)
	}
	state.Modules, err = rm.FindModules(ctx)
	if err != nil {
		return nil, fmt.Errorf("find modules: %w", err)
	}
	state.CurrentVersion = rm.GetCurrentGlobalVersion()
	state.TagCache = rm.tagCache

	return state, nil
}

// ShowCurrentState displays current release state without version argument
func (rm *Manager) ShowCurrentState(ctx context.Context) error {
	state, err := rm.AssessCurrentState(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Current Repository State\nbranch: %s\nglobal_version: %s\n", state.CurrentBranch, state.CurrentVersion)

	fmt.Println("Modules and their current versions:")
	for _, module := range state.Modules {
		moduleName := module.Path
		if moduleName == "" {
			moduleName = "root"
		}
		fmt.Printf("%s - %s\n", moduleName, module.Version)
	}

	return nil
}

// InteractiveRelease performs interactive release with confirmations
func (rm *Manager) InteractiveRelease(ctx context.Context, targetVersion string) error {
	// 1. Assess state (assumes cache is already populated)
	state, err := rm.AssessCurrentState(ctx)
	if err != nil {
		return err
	}

	actions, warnings := rm.planReleaseActions(state, targetVersion)

	if err = rm.handleWarningsAndConfirmations(ctx, warnings); err != nil {
		return err
	}

	rm.ShowPlannedActions(actions)

	if !rm.config.SkipConfirmations {
		confirmed, err := rm.interaction.Confirm(ctx, "Create tags?")
		if err != nil || !confirmed {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("tag creation cancelled")
		}
	}

	if err = rm.executeTagCreation(ctx, actions); err != nil {
		return err
	}

	if !rm.config.SkipConfirmations {
		confirmed, err := rm.interaction.Confirm(ctx, "Push tags?")
		if err != nil || !confirmed {
			if ctx.Err() != nil {
				return nil
			}
			fmt.Println("Tags created locally but not pushed")
			return nil
		}
	}
	if err = rm.executeTagPushing(ctx, actions); err != nil {
		return fmt.Errorf("push tags: %w", err)
	}
	fmt.Println("Release completed successfully")
	return nil
}

// ShowPlannedActions displays what actions will be performed
func (rm *Manager) ShowPlannedActions(actions []Action) {
	if len(actions) == 0 {
		fmt.Println("No actions planned")
		return
	}

	fmt.Println("Planned Release Actions")

	// Group actions by type for better readability
	createActions := make([]Action, 0)
	pushActions := make([]Action, 0)

	for _, action := range actions {
		switch action.Type {
		case ActionCreateTag:
			createActions = append(createActions, action)
		case ActionPushTags:
			pushActions = append(pushActions, action)
		}
	}
	fmt.Println()
	for _, action := range createActions {
		fmt.Printf("git tag %s\n", action.Target)
	}
	fmt.Println()
	for _, action := range pushActions {
		fmt.Printf("git push origin %s\n", action.Target)
	}

	return
}

func (rm *Manager) planReleaseActions(state *State, targetVersion string) ([]Action, []Warning) {
	var actions []Action
	var warnings []Warning

	// Add actions for each module
	for _, module := range state.Modules {
		tagName := rm.getTagName(module, targetVersion)

		actions = append(actions, Action{
			Type:        ActionCreateTag,
			Target:      tagName,
			Description: fmt.Sprintf("Create tag %s", tagName),
		})

		actions = append(actions, Action{
			Type:        ActionPushTags,
			Target:      tagName,
			Description: fmt.Sprintf("Push tag %s", tagName),
		})
	}

	// Add warnings
	warnings = append(warnings, rm.validateWithWarnings(state, targetVersion)...)

	return actions, warnings
}

func (rm *Manager) validateWithWarnings(state *State, targetVersion string) []Warning {
	var warnings []Warning

	// Branch check -> warning
	if rm.config.RequiredBranch != "" && state.CurrentBranch != rm.config.RequiredBranch {
		warnings = append(warnings, Warning{
			Type:    WrongBranch,
			Message: fmt.Sprintf("you are not on %s", rm.config.RequiredBranch),
		})
	}

	// Existing tags -> warning
	if err := rm.CheckVersionExists(targetVersion, state.Modules); err != nil {
		warnings = append(warnings, Warning{
			Type:    ExistingTags,
			Message: fmt.Sprintf("these tags already exist: %v", err),
		})
	}

	return warnings
}

// getLatestVersionForModule returns the latest version for a given module path
func (rm *Manager) getLatestVersionForModule(modulePath string) string {
	// Handle case where tag cache isn't initialized yet
	if rm.tagCache == nil || rm.tagCache.ModuleTags == nil {
		return "v0.0.0" // Default for modules with no releases yet
	}

	moduleTags, exists := rm.tagCache.ModuleTags[modulePath]
	if !exists || len(moduleTags) == 0 {
		return "v0.0.0" // No releases for this module yet
	}

	// Find the latest version among all tags for this module
	var latestVersion *semver.Version
	for _, tag := range moduleTags {
		if tag.Version != nil {
			if latestVersion == nil || tag.Version.GreaterThan(latestVersion) {
				latestVersion = tag.Version
			}
		}
	}

	if latestVersion == nil {
		return "v0.0.0"
	}

	return "v" + latestVersion.String()
}

func (rm *Manager) handleWarningsAndConfirmations(ctx context.Context, warnings []Warning) error {
	for _, warning := range warnings {
		fmt.Println(warning.Message)

		if rm.config.SkipConfirmations {
			return nil
		}

		confirmed, err := rm.interaction.ConfirmWithDefault(ctx, "Continue?", false)
		if err != nil {
			return err
		}
		if !confirmed {
			return fmt.Errorf("operation cancelled due to: %s", warning.Message)
		}
	}
	return nil
}

func (rm *Manager) executeTagCreation(ctx context.Context, actions []Action) error {
	fmt.Println("Creating tags...")
	for _, action := range actions {
		if action.Type == ActionCreateTag {
			if err := rm.git.CreateTag(ctx, action.Target); err != nil {
				return fmt.Errorf("failed to create tag %s: %w", action.Target, err)
			}
			fmt.Printf("Created tag %s\n", action.Target)
		}
	}
	return nil
}

func (rm *Manager) executeTagPushing(ctx context.Context, actions []Action) error {
	fmt.Println("Pushing tags...")
	for _, action := range actions {
		if action.Type == ActionPushTags {
			if err := rm.git.PushTag(ctx, action.Target); err != nil {
				return fmt.Errorf("failed to push tag %s: %w", action.Target, err)
			}
			fmt.Printf("Pushed tag %s\n", action.Target)
		}
	}
	return nil
}

// CalculateNewVersion returns new version
func (rm *Manager) calculateNewVersion(currentVersionStr string) (string, error) {
	rm.logDebug("Calculating new version\n current:%s\ntype:%s\nprerelease:%t\n", currentVersionStr, rm.config.VersionType, rm.config.Prerelease)

	if rm.config.Version != "" {
		// Explicit version specified
		newVersion, err := NormalizeVersion(rm.config.Version)
		if err != nil {
			return "", fmt.Errorf("invalid version format: %w", err)
		}

		// Check if we need to add prerelease suffix
		parsedVersion, err := semver.NewVersion(newVersion)
		if err != nil {
			return "", fmt.Errorf("failed to parse new version: %w", err)
		}

		if rm.config.Prerelease && parsedVersion.Prerelease() == "" {
			return rm.GetNextPrereleaseVersion(newVersion)
		}

		return newVersion, nil
	}

	if rm.config.VersionType == "prerelease-only" {
		// Just increment prerelease number
		currentVersion, err := semver.NewVersion(currentVersionStr)
		if err != nil {
			return "", fmt.Errorf("failed to parse current version: %w", err)
		}

		if currentVersion.Prerelease() != "" {
			// Extract base version from current prerelease
			baseVersionStr := fmt.Sprintf("v%d.%d.%d", currentVersion.Major(), currentVersion.Minor(), currentVersion.Patch())
			return rm.GetNextPrereleaseVersion(baseVersionStr)
		}
		// Make first prerelease of current version
		return rm.GetNextPrereleaseVersion(currentVersionStr)
	}

	// Increment based on type
	newVersion, err := IncrementVersion(currentVersionStr, rm.config.VersionType)
	if err != nil {
		return "", err
	}

	if rm.config.Prerelease {
		return rm.GetNextPrereleaseVersion(newVersion)
	}

	return newVersion, nil
}

// NormalizeVersion ensures version has 'v' prefix and is valid semver
func NormalizeVersion(v string) (string, error) {
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}

	// Parse with Masterminds/semver to validate
	_, err := semver.NewVersion(v)
	if err != nil {
		return "", fmt.Errorf("invalid semantic version: %s", v)
	}

	return v, nil
}

// IncrementVersion increments a version based on type
func IncrementVersion(currentVersionStr, versionType string) (string, error) {
	// Parse the current version
	currentVersion, err := semver.NewVersion(currentVersionStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse current version %s: %w", currentVersionStr, err)
	}

	var newVersion semver.Version
	switch versionType {
	case "major":
		newVersion = currentVersion.IncMajor()
	case "minor":
		newVersion = currentVersion.IncMinor()
	case "patch":
		newVersion = currentVersion.IncPatch()
	default:
		return "", fmt.Errorf("invalid version type: %s", versionType)
	}

	return "v" + newVersion.String(), nil
}

// FindModules discovers all Go modules in the repository
func (rm *Manager) FindModules(ctx context.Context) ([]Module, error) {
	rm.logDebug("Discovering Go modules in path %s", rm.config.RepoRoot)
	goModPaths, err := rm.fs.FindGoModFiles(ctx, rm.config.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to find go.mod files: %w", err)
	}

	var modules []Module
	seen := make(map[string]bool)

	for _, path := range goModPaths {

		// Normalize relative path
		if path == "." {
			path = ""
		}
		path = strings.TrimPrefix(path, "./")

		// Check if should be excluded
		if rm.shouldExcludeModule(path) {
			rm.logDebug("Excluding module %s", path)
			continue
		}

		// Deduplicate
		if seen[path] {
			continue
		}
		seen[path] = true

		// Get latest version for this module from cache
		latestVersion := rm.getLatestVersionForModule(path)

		modules = append(modules, Module{
			Path:    path,
			Version: latestVersion,
		})
		rm.logDebug("Found module\n%s\nversion: %s\n", path, latestVersion)
	}

	return modules, nil
}

// shouldExcludeModule checks if a module should be excluded
func (rm *Manager) shouldExcludeModule(relPath string) bool {
	for _, excluded := range rm.config.ExcludedDirs {
		if relPath == excluded || strings.HasPrefix(relPath, excluded+"/") {
			return true
		}
	}
	return false
}

// getTagName generates the tag name for a module and version
func (rm *Manager) getTagName(module Module, version string) string {
	if module.Path == "" {
		return version
	}
	return module.Path + "/" + version
}

func (rm *Manager) logDebug(msg string, args ...interface{}) {
	if rm.config.Verbose {
		fmt.Printf("%s\n", fmt.Sprintf(msg, args...))
	}
}
