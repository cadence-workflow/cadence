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
	RepoRoot          string
	ExcludedDirs      []string
	RequiredBranch    string
	Verbose           bool
	Command           string // The subcommand being executed
	SkipConfirmations bool
	ManualVersion     string // Manual version override
}

// Manager handles the release process
type Manager struct {
	config      Config
	git         Git
	fs          FS
	interaction UserInteraction
	tagCache    *TagCache
}

func NewReleaseManager(config Config, git Git, fs FS, interaction UserInteraction) *Manager {
	return &Manager{
		config:      config,
		git:         git,
		fs:          fs,
		interaction: interaction,
	}
}

func (rm *Manager) RunRelease(ctx context.Context) error {
	return rm.executeCommand(ctx, rm.calculateReleaseVersion, rm.validateReleaseCommand)
}

func (rm *Manager) RunPatch(ctx context.Context) error {
	return rm.executeCommand(ctx, rm.calculatePatchVersion, rm.validateMinorMajorCommand)
}

func (rm *Manager) RunMinor(ctx context.Context) error {
	return rm.executeCommand(ctx, rm.calculateMinorVersion, rm.validateMinorMajorCommand)
}

func (rm *Manager) RunMajor(ctx context.Context) error {
	return rm.executeCommand(ctx, rm.calculateMajorVersion, rm.validateMinorMajorCommand)
}

func (rm *Manager) RunPrerelease(ctx context.Context) error {
	return rm.executeCommand(ctx, rm.calculatePrereleaseVersion, nil)
}

// Generic command execution flow
func (rm *Manager) executeCommand(
	ctx context.Context,
	calculateVersion func(string) (string, error),
	validate func(string) error,
) error {
	// Initialize tag cache
	if err := rm.GetKnownReleases(ctx); err != nil {
		return err
	}

	// Get current version
	currentVersion := rm.GetCurrentGlobalVersion()
	rm.logDebug("Current version: %s", currentVersion)

	var targetVersion string
	var err error

	// Check for manual version override
	if rm.config.ManualVersion != "" {
		rm.logDebug("Using manual version override: %s", rm.config.ManualVersion)
		targetVersion, err = rm.processManualVersion(rm.config.ManualVersion)
		if err != nil {
			return err
		}
	} else {
		// Run validation if provided (only for automatic version calculation)
		if validate != nil {
			if err := validate(currentVersion); err != nil {
				return err
			}
		}

		// Calculate target version automatically
		targetVersion, err = calculateVersion(currentVersion)
		if err != nil {
			return err
		}
	}

	fmt.Printf("Version transition: %s → %s\n", currentVersion, targetVersion)

	return rm.executeRelease(ctx, targetVersion)
}

// Version calculation methods for each subcommand
func (rm *Manager) calculateReleaseVersion(currentVersionStr string) (string, error) {
	rm.logDebug("Calculating release version from: %s", currentVersionStr)

	currentVersion, err := semver.NewVersion(currentVersionStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse current version %s: %w", currentVersionStr, err)
	}

	// Remove prerelease suffix
	releaseVersion := fmt.Sprintf("v%d.%d.%d",
		currentVersion.Major(),
		currentVersion.Minor(),
		currentVersion.Patch())

	return releaseVersion, nil
}

// Generic version calculation that creates a closure for different version types
func (rm *Manager) calculateVersionIncrement(versionType string) func(string) (string, error) {
	return func(currentVersionStr string) (string, error) {
		rm.logDebug("Calculating %s version from: %s", versionType, currentVersionStr)

		// First, get the base version (remove prerelease if present)
		baseVersion := rm.getBaseVersion(currentVersionStr)

		// Increment version
		newVersion, err := IncrementVersion(baseVersion, versionType)
		if err != nil {
			return "", err
		}

		// Create first prerelease
		return rm.GetNextPrereleaseVersion(newVersion)
	}
}

// Update the existing methods to use the generic function
func (rm *Manager) calculateMinorVersion(currentVersionStr string) (string, error) {
	return rm.calculateVersionIncrement("minor")(currentVersionStr)
}

func (rm *Manager) calculateMajorVersion(currentVersionStr string) (string, error) {
	return rm.calculateVersionIncrement("major")(currentVersionStr)
}

func (rm *Manager) calculatePatchVersion(currentVersionStr string) (string, error) {
	return rm.calculateVersionIncrement("patch")(currentVersionStr)
}

func (rm *Manager) calculatePrereleaseVersion(currentVersionStr string) (string, error) {
	rm.logDebug("Calculating prerelease version from: %s", currentVersionStr)

	currentVersion, err := semver.NewVersion(currentVersionStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse current version %s: %w", currentVersionStr, err)
	}

	baseVersionStr := fmt.Sprintf("v%d.%d.%d",
		currentVersion.Major(),
		currentVersion.Minor(),
		currentVersion.Patch())

	return rm.GetNextPrereleaseVersion(baseVersionStr)
}

// Validation methods
func (rm *Manager) validateReleaseCommand(currentVersion string) error {
	if !strings.Contains(currentVersion, "-prerelease") {
		return fmt.Errorf("release command requires existing prerelease version, current: %s", currentVersion)
	}
	return nil
}

func (rm *Manager) validateMinorMajorCommand(currentVersion string) error {
	if strings.Contains(currentVersion, "-prerelease") {
		return fmt.Errorf("minor/major commands should be run from stable versions, current: %s (consider using 'release' first)", currentVersion)
	}
	return nil
}

// Helper method to get base version (remove prerelease suffix)
func (rm *Manager) getBaseVersion(versionStr string) string {
	version, err := semver.NewVersion(versionStr)
	if err != nil {
		return versionStr
	}

	return fmt.Sprintf("v%d.%d.%d", version.Major(), version.Minor(), version.Patch())
}

// processManualVersion validates and normalizes manual version input
func (rm *Manager) processManualVersion(manualVersion string) (string, error) {
	// Normalize the version (ensure v prefix, validate semver)
	normalizedVersion, err := NormalizeVersion(manualVersion)
	if err != nil {
		return "", fmt.Errorf("invalid manual version format: %w", err)
	}

	rm.logDebug("Manual version normalized: %s → %s", manualVersion, normalizedVersion)
	return normalizedVersion, nil
}

// Execute the actual release
func (rm *Manager) executeRelease(ctx context.Context, targetVersion string) error {
	// Assess state
	state, err := rm.AssessCurrentState(ctx)
	if err != nil {
		return err
	}

	// Plan actions
	actions, warnings := rm.planReleaseActions(state, targetVersion)

	// Handle warnings and get confirmation to continue
	if err = rm.handleWarningsAndConfirmations(ctx, warnings); err != nil {
		return err
	}

	// Check for version conflicts and handle them
	conflictInfo, conflictErr := rm.CheckVersionExists(targetVersion, state.Modules)
	finalActions := actions

	if conflictErr != nil {
		fmt.Printf("❌ Version conflict detected:\n")
		fmt.Printf("   Existing tags: %v\n", conflictInfo.ExistingTags)
		fmt.Printf("   Tags to create: %v\n", conflictInfo.MissingTags)

		if len(conflictInfo.MissingTags) == 0 {
			return fmt.Errorf("all tags already exist for version %s", targetVersion)
		}

		if !rm.config.SkipConfirmations {
			confirmed, err := rm.interaction.ConfirmWithDefault(ctx,
				fmt.Sprintf("Continue and create only missing tags (%d remaining)?", len(conflictInfo.MissingTags)),
				false)
			if err != nil || !confirmed {
				return fmt.Errorf("operation cancelled due to version conflict")
			}
		}

		// Filter actions to only include missing tags
		finalActions = rm.filterActionsForMissingTags(actions, conflictInfo.MissingTags)
		fmt.Printf("✓ Will skip existing tags and create only: %v\n", conflictInfo.MissingTags)
	}

	// Show planned actions (filtered if there were conflicts)
	rm.ShowPlannedActions(finalActions)

	// Confirm tag creation
	if !rm.config.SkipConfirmations {
		tagCount := len(finalActions) / 2 // Each tag has create + push action
		message := fmt.Sprintf("Create %d tags?", tagCount)
		if conflictErr != nil {
			message = fmt.Sprintf("Create %d missing tags (skipping %d existing)?", len(conflictInfo.MissingTags), len(conflictInfo.ExistingTags))
		}

		confirmed, err := rm.interaction.Confirm(ctx, message)
		if err != nil || !confirmed {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("tag creation cancelled")
		}
	}

	// Create tags (only missing ones if there were conflicts)
	if err = rm.executeTagCreation(ctx, finalActions); err != nil {
		return err
	}

	// Confirm tag pushing
	if !rm.config.SkipConfirmations {
		pushCount := 0
		for _, action := range finalActions {
			if action.Type == ActionPushTags {
				pushCount++
			}
		}

		message := fmt.Sprintf("Push %d tags?", pushCount)
		confirmed, err := rm.interaction.Confirm(ctx, message)
		if err != nil || !confirmed {
			if ctx.Err() != nil {
				return nil
			}
			fmt.Printf("Tags created locally but not pushed\n")
			if conflictErr != nil {
				fmt.Printf("Created: %v\n", conflictInfo.MissingTags)
				fmt.Printf("Skipped: %v\n", conflictInfo.ExistingTags)
			}
			return nil
		}
	}

	// Push tags
	if err = rm.executeTagPushing(ctx, finalActions); err != nil {
		return fmt.Errorf("push tags: %w", err)
	}

	// Success message
	if conflictErr != nil {
		fmt.Printf("✓ Release %s completed successfully\n", targetVersion)
		fmt.Printf("  Created: %v\n", conflictInfo.MissingTags)
		fmt.Printf("  Skipped: %v (already existed)\n", conflictInfo.ExistingTags)
	} else {
		fmt.Printf("✓ Release %s completed successfully\n", targetVersion)
	}

	return nil
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

	if nextNum > 99 {
		return "", fmt.Errorf("maximum prerelease number (99) exceeded, base (%s)", cleanBaseStr)
	}

	return fmt.Sprintf("%s-prerelease%02d", cleanBaseStr, nextNum), nil
}

func (rm *Manager) CheckVersionExists(version string, modules []Module) (VersionConflictInfo, error) {
	conflictInfo := VersionConflictInfo{
		ExistingTags: make([]string, 0),
		MissingTags:  make([]string, 0),
	}

	for _, module := range modules {
		expectedTag := version
		if module.Path != "" {
			expectedTag = module.Path + "/" + version
		}

		exists := false
		for _, tag := range rm.tagCache.AllTags {
			if tag.Raw == expectedTag {
				conflictInfo.ExistingTags = append(conflictInfo.ExistingTags, expectedTag)
				exists = true
				break
			}
		}

		if !exists {
			conflictInfo.MissingTags = append(conflictInfo.MissingTags, expectedTag)
		}
	}

	if len(conflictInfo.ExistingTags) > 0 {
		return conflictInfo, fmt.Errorf("some tags already exist: %v", conflictInfo.ExistingTags)
	}

	return conflictInfo, nil
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

// ShowCurrentState displays current release state
func (rm *Manager) ShowCurrentState(ctx context.Context) error {
	// Initialize tag cache
	if err := rm.GetKnownReleases(ctx); err != nil {
		return err
	}

	state, err := rm.AssessCurrentState(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Repository Release Status\n")
	fmt.Printf("========================\n")
	fmt.Printf("Branch: %s\n", state.CurrentBranch)
	fmt.Printf("Global Version: %s\n", state.CurrentVersion)
	fmt.Printf("\n")

	fmt.Printf("Modules and Versions:\n")
	for _, module := range state.Modules {
		moduleName := module.Path
		if moduleName == "" {
			moduleName = "root"
		}
		fmt.Printf("  %-20s %s\n", moduleName, module.Version)
	}

	// Show what commands are available
	fmt.Printf("\nAvailable Commands:\n")
	if strings.Contains(state.CurrentVersion, "-prerelease") {
		fmt.Printf("  releaser prerelease              # Increment prerelease number\n")
		fmt.Printf("  releaser release                 # Promote to final release\n")
		fmt.Printf("  releaser release -s v1.4.0       # Override with specific version\n")
	} else {
		fmt.Printf("  releaser minor                   # Start new minor version cycle\n")
		fmt.Printf("  releaser major                   # Start new major version cycle\n")
		fmt.Printf("  releaser patch                   # Start new patch version cycle\n")
		fmt.Printf("  releaser minor -s v1.4.0-prerelease01  # Override with specific version\n")
	}

	return nil
}

// ShowPlannedActions displays what actions will be performed
func (rm *Manager) ShowPlannedActions(actions []Action) {
	if len(actions) == 0 {
		fmt.Println("No actions planned")
		return
	}

	fmt.Println("\nPlanned Release Actions:")

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

	if len(createActions) > 0 {
		fmt.Println("\nCreate Tags:")
		for _, action := range createActions {
			fmt.Printf("  git tag %s\n", action.Target)
		}
	}

	if len(pushActions) > 0 {
		fmt.Println("\nPush Tags:")
		for _, action := range pushActions {
			fmt.Printf("  git push origin %s\n", action.Target)
		}
	}
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

	return warnings
}

// filterActionsForMissingTags filters actions to only include missing tags
func (rm *Manager) filterActionsForMissingTags(actions []Action, missingTags []string) []Action {
	var filteredActions []Action

	// Create a set of missing tags for quick lookup
	missingTagSet := make(map[string]bool)
	for _, tag := range missingTags {
		missingTagSet[tag] = true
	}

	// Filter actions to only include missing tags
	for _, action := range actions {
		if missingTagSet[action.Target] {
			filteredActions = append(filteredActions, action)
		}
	}

	return filteredActions
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
		fmt.Printf("⚠️  %s\n", warning.Message)

		if rm.config.SkipConfirmations {
			continue
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
			fmt.Printf("✓ Created tag %s\n", action.Target)
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
			fmt.Printf("✓ Pushed tag %s\n", action.Target)
		}
	}
	return nil
}

// NormalizeVersion ensures version has 'v' prefix and is valid semver
func NormalizeVersion(v string) (string, error) {
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}

	// Parse with Masterminds/semver to validate
	semVer, err := semver.NewVersion(v)
	if err != nil {
		return "", fmt.Errorf("invalid semantic version: %s", v)
	}

	return "v" + semVer.String(), nil
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
		fmt.Printf("DEBUG: %s\n", fmt.Sprintf(msg, args...))
	}
}
