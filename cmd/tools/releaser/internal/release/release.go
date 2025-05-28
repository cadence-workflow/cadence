package release

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"go.uber.org/zap"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination release_mocks_test.go -self_package github.com/uber/cadence/cmd/tools/releaser/release

// Git defines git operations for testing
type Git interface {
	GetCurrentBranch() (string, error)
	IsWorkingDirClean() (bool, error)
	GetTags() ([]string, error)
	CreateTag(tag string) error
	PushTag(tag string) error
	GetRepoRoot() (string, error)
}

// FS defines filesystem operations for testing
type FS interface {
	FindGoModFiles(root string) ([]string, error)
	ModTidy(dir string) error
	Build(dir string) error
}

// Config holds the release configuration
type Config struct {
	RepoRoot       string
	DryRun         bool
	Prerelease     bool
	Version        string
	VersionType    string
	ExcludedDirs   []string
	RequiredBranch string
}

// Manager handles the release process
type Manager struct {
	config *Config
	git    Git
	fs     FS
	logger *zap.Logger
}

func NewReleaseManager(config *Config, git Git, fs FS, logger *zap.Logger) *Manager {
	return &Manager{
		config: config,
		git:    git,
		fs:     fs,
		logger: logger,
	}
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
func (rm *Manager) FindModules() ([]Module, error) {
	rm.logger.Info("Discovering Go modules", zap.String("root", rm.config.RepoRoot))
	goModPaths, err := rm.fs.FindGoModFiles(rm.config.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to find go.mod files: %w", err)
	}

	var modules []Module
	seen := make(map[string]bool)

	for _, path := range goModPaths {
		relPath, err := filepath.Rel(rm.config.RepoRoot, path)
		if err != nil {
			continue
		}

		// Normalize relative path
		if relPath == "." {
			relPath = ""
		}
		relPath = strings.TrimPrefix(relPath, "./")

		// Check if should be excluded
		if rm.shouldExcludeModule(relPath) {
			rm.logger.Debug("Excluding module", zap.String("path", relPath))
			continue
		}

		// Deduplicate
		if seen[path] {
			continue
		}
		seen[path] = true

		modules = append(modules, Module{
			Path:         path,
			RelativePath: relPath,
		})
		rm.logger.Debug("Found module", zap.String("path", path), zap.String("relative", relPath))
	}

	rm.logger.Info("Module discovery complete", zap.Int("count", len(modules)))
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

// GetCurrentGlobalVersion finds the highest version across all modules
func (rm *Manager) GetCurrentGlobalVersion() (string, error) {
	rm.logger.Info("Determining current global version")
	tags, err := rm.git.GetTags()
	if err != nil {
		return "", fmt.Errorf("failed to get git tags: %w", err)
	}

	var validVersions []*semver.Version
	versionRegex := regexp.MustCompile(`v[0-9]+\.[0-9]+\.[0-9]+(?:-prerelease[0-9]+)?`)

	for _, tag := range tags {
		// Extract version part from tag (remove module prefix if present)
		versionPart := tag
		if idx := strings.LastIndex(tag, "/v"); idx != -1 {
			versionPart = tag[idx+1:]
		}

		if versionRegex.MatchString(versionPart) {
			if version, err := semver.NewVersion(versionPart); err == nil {
				validVersions = append(validVersions, version)
			}
		}
	}

	if len(validVersions) == 0 {
		rm.logger.Info("No existing versions found, starting from v0.0.0")
		return "v0.0.0", nil
	}

	// Sort versions using semver.Collection and return the highest
	collection := semver.Collection(validVersions)
	sort.Sort(collection)
	currentVersion := "v" + collection[len(collection)-1].String()

	rm.logger.Info("Current global version determined", zap.String("version", currentVersion))
	return currentVersion, nil
}

// GetNextPrereleaseVersion generates the next prerelease version
func (rm *Manager) GetNextPrereleaseVersion(baseVersionStr string) (string, error) {
	rm.logger.Info("Calculating next prerelease version", zap.String("base", baseVersionStr))

	// Parse base version and ensure it doesn't have prerelease suffix
	baseVersion, err := semver.NewVersion(baseVersionStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse base version %s: %w", baseVersionStr, err)
	}

	// Create clean base version without prerelease
	cleanBaseStr := fmt.Sprintf("v%d.%d.%d", baseVersion.Major(), baseVersion.Minor(), baseVersion.Patch())

	tags, err := rm.git.GetTags()
	if err != nil {
		return "", fmt.Errorf("failed to get git tags: %w", err)
	}

	baseVersionWithoutV := strings.TrimPrefix(cleanBaseStr, "v")
	prereleaseRegex := regexp.MustCompile(fmt.Sprintf(`v%s-prerelease(\d+)`, regexp.QuoteMeta(baseVersionWithoutV)))

	// Find existing prereleases for this base version
	var prereleaseNumbers []int
	var hasSingleDigitFormat bool

	for _, tag := range tags {
		// Check both direct tags and module-prefixed tags
		tagVersions := []string{tag}
		if idx := strings.LastIndex(tag, "/v"); idx != -1 {
			tagVersions = append(tagVersions, tag[idx+1:])
		}

		for _, tagVersion := range tagVersions {
			matches := prereleaseRegex.FindStringSubmatch(tagVersion)
			if len(matches) > 1 {
				if num, err := strconv.Atoi(matches[1]); err == nil {
					prereleaseNumbers = append(prereleaseNumbers, num)
					// Check if this uses single-digit format (no leading zero for numbers < 10)
					if num < 10 && len(matches[1]) == 1 {
						hasSingleDigitFormat = true
					}
				}
			}
		}
	}

	// If any single-digit format found, error out
	if hasSingleDigitFormat {
		sort.Ints(prereleaseNumbers)
		for _, num := range prereleaseNumbers {
			if num < 10 {
				return "", fmt.Errorf("latest prerelease uses 1-digit format. Only 2-digit format supported, base (%s), number (%d)", cleanBaseStr, num)
			}
		}
	}

	// Find the next prerelease number
	nextNum := 1
	if len(prereleaseNumbers) > 0 {
		sort.Ints(prereleaseNumbers)
		nextNum = prereleaseNumbers[len(prereleaseNumbers)-1] + 1
	}

	if nextNum > 99 {
		return "", fmt.Errorf("maximum prerelease number (99) exceeded, base (%s)", cleanBaseStr)
	}

	newVersion := fmt.Sprintf("%s-prerelease%02d", cleanBaseStr, nextNum)

	rm.logger.Info("Next prerelease version calculated", zap.String("version", newVersion))
	return newVersion, nil
}

// CalculateNewVersion determines the new version based on configuration
func (rm *Manager) CalculateNewVersion(currentVersionStr string) (string, error) {
	rm.logger.Info("Calculating new version",
		zap.String("current", currentVersionStr),
		zap.String("type", rm.config.VersionType),
		zap.Bool("prerelease", rm.config.Prerelease))

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
		} else {
			// Make first prerelease of current version
			return rm.GetNextPrereleaseVersion(currentVersionStr)
		}
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

// CheckVersionExists verifies that the version doesn't already exist
func (rm *Manager) CheckVersionExists(version string, modules []Module) error {
	rm.logger.Info("Checking if version already exists", zap.String("version", version))
	tags, err := rm.git.GetTags()
	if err != nil {
		return fmt.Errorf("failed to get git tags: %w", err)
	}

	tagSet := make(map[string]bool)
	for _, tag := range tags {
		tagSet[tag] = true
	}

	var existingTags []string
	for _, module := range modules {
		var expectedTag string
		if module.RelativePath == "" {
			expectedTag = version
		} else {
			expectedTag = module.RelativePath + "/" + version
		}

		if tagSet[expectedTag] {
			existingTags = append(existingTags, expectedTag)
		}
	}

	if len(existingTags) > 0 {
		return fmt.Errorf("version already exists for modules: %v", existingTags)
	}

	rm.logger.Info("Version is available", zap.String("version", version))
	return nil
}

// ValidateEnvironment checks that we're in the right state to release
func (rm *Manager) ValidateEnvironment() error {
	rm.logger.Info("Validating environment for release")

	// Check branch
	if rm.config.RequiredBranch != "" {
		branch, err := rm.git.GetCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}

		if branch != rm.config.RequiredBranch {
			return fmt.Errorf("must be on %s branch, currently on: %s", rm.config.RequiredBranch, branch)
		}
		rm.logger.Info("Branch validation passed", zap.String("branch", branch))
	}

	// Check working directory (skip in dry-run)
	if !rm.config.DryRun {
		clean, err := rm.git.IsWorkingDirClean()
		if err != nil {
			return fmt.Errorf("failed to check working directory: %w", err)
		}

		if !clean {
			return fmt.Errorf("working directory is not clean. Please commit or stash changes")
		}
		rm.logger.Info("Working directory is clean")
	}

	return nil
}

// CreateRelease creates releases for all modules
func (rm *Manager) CreateRelease(modules []Module, version string) error {
	rm.logger.Info("Starting release creation",
		zap.String("version", version),
		zap.Int("modules", len(modules)))

	rm.logger.Info("Creating and pushing tags:")

	var failedModules []string

	for _, module := range modules {
		if err := rm.createModuleRelease(module, version); err != nil {
			failedModules = append(failedModules, module.RelativePath)
			rm.logger.Info(rm.getTagName(module, version), zap.Error(err))
		}
	}

	if len(failedModules) > 0 {
		return fmt.Errorf("failed to process %d module(s): %v", len(failedModules), failedModules)
	}

	rm.logger.Info("Release creation completed successfully")
	return nil
}

// createModuleRelease creates a release for a single module
func (rm *Manager) createModuleRelease(module Module, version string) error {
	tagName := rm.getTagName(module, version)
	rm.logger.Info("Creating module release",
		zap.String("module", module.RelativePath),
		zap.String("tag", tagName))

	// Build and test module
	if err := rm.fs.ModTidy(module.Path); err != nil {
		return fmt.Errorf("mod tidy failed: %w", err)
	}

	if err := rm.fs.Build(module.Path); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Create and push tag
	if err := rm.git.CreateTag(tagName); err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}

	if err := rm.git.PushTag(tagName); err != nil {
		return fmt.Errorf("failed to push tag: %w", err)
	}

	rm.logger.Info("Module release completed", zap.String("tag", tagName))
	return nil
}

// getTagName generates the tag name for a module and version
func (rm *Manager) getTagName(module Module, version string) string {
	if module.RelativePath == "" {
		return version
	}
	return module.RelativePath + "/" + version
}

// Run executes the release process
func (rm *Manager) Run() error {
	rm.logger.Info("Starting release process",
		zap.String("version", rm.config.Version),
		zap.String("type", rm.config.VersionType),
		zap.Bool("prerelease", rm.config.Prerelease),
		zap.Bool("dry_run", rm.config.DryRun))

	if rm.config.DryRun {
		rm.logger.Info("⚠️  DRY RUN MODE - No changes will be made")
	}

	// Validate environment
	if err := rm.ValidateEnvironment(); err != nil {
		return err
	}

	// Find modules
	modules, err := rm.FindModules()
	if err != nil {
		return err
	}

	if len(modules) == 0 {
		return fmt.Errorf("no Go modules found")
	}

	// Get current version
	currentVersion, err := rm.GetCurrentGlobalVersion()
	if err != nil {
		return err
	}

	// Calculate new version
	newVersion, err := rm.CalculateNewVersion(currentVersion)
	if err != nil {
		return err
	}

	// Check if version already exists
	if err := rm.CheckVersionExists(newVersion, modules); err != nil {
		return err
	}

	// Show tags that will be created
	tagsCreated := make([]string, 0, len(modules))
	for _, module := range modules {
		tagsCreated = append(tagsCreated, rm.getTagName(module, newVersion))
	}
	rm.logger.Info("Tags to be created", zap.Strings("tags", tagsCreated))

	if rm.config.DryRun {
		rm.logger.Info("Dry run completed successfully")
		return nil
	}

	// Create releases
	if err := rm.CreateRelease(modules, newVersion); err != nil {
		return err
	}

	rm.logger.Info("Release process completed successfully", zap.String("version", newVersion))
	return nil
}

// Module represents a Go module
type Module struct {
	Path         string
	RelativePath string
}
