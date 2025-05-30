package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/cmd/tools/releaser/internal/console"
	"github.com/uber/cadence/cmd/tools/releaser/internal/fs"
	"github.com/uber/cadence/cmd/tools/releaser/internal/git"
	"github.com/uber/cadence/cmd/tools/releaser/internal/release"
)

// CLI handling - CLI creates fx.App
func main() {
	cliApp := &cli.App{
		Name:    "release",
		Usage:   "Create releases for all Go modules in the repository with unified versioning",
		Version: "1.0.0",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "type",
				Aliases: []string{"t"},
				Usage:   "Version bump type: major, minor, patch",
			},
			&cli.StringFlag{
				Name:    "set-version",
				Aliases: []string{"s"},
				Usage:   "Specific version to set for all modules (overrides --type)",
			},
			&cli.BoolFlag{
				Name:    "prerelease",
				Aliases: []string{"p"},
				Usage:   "Create prerelease versions (adds -prereleaseNN suffix). Can be used alone to increment prerelease number only",
			},
			&cli.BoolFlag{
				Name:    "dry-run",
				Aliases: []string{"d"},
				Usage:   "Show what would be done without making changes",
				Value:   true,
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"i"},
			},
		},
		Action: runRelease,
		Commands: []*cli.Command{
			{
				Name:  "examples",
				Usage: "Show usage examples",
				Action: func(c *cli.Context) error {
					showExamples()
					return nil
				},
			},
		},
		CustomAppHelpTemplate: `NAME:
   {{.Name}} - {{.Usage}}

USAGE:
   {{.HelpName}} [global options]

VERSION:
   {{.Version}}

GLOBAL OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}

EXAMPLES:
   {{.HelpName}} --type patch                        # Bump patch version for all modules
   {{.HelpName}} --type minor --prerelease           # Bump minor version with prerelease suffix
   {{.HelpName}} --set-version v1.2.3               # Set v1.2.3 for all modules
   {{.HelpName}} --set-version v1.2.3 --prerelease  # Set v1.2.3-prerelease01 for all modules
   {{.HelpName}} --prerelease                       # Just increment prerelease number (no version bump)
   {{.HelpName}} --type major --dry-run              # Show what would happen with major bump

PRERELEASE WORKFLOWS:
   # Start prerelease cycle for v1.2.4
   {{.HelpName}} --type patch --prerelease           # Creates v1.2.4-prerelease01
   
   # Continue prerelease iterations (no version bump)
   {{.HelpName}} --prerelease                       # Creates v1.2.4-prerelease02
   {{.HelpName}} --prerelease                       # Creates v1.2.4-prerelease03
   
   # Final release
   {{.HelpName}} --set-version v1.2.4               # Creates v1.2.4 (removes prerelease suffix)

PRERELEASE FORMAT:
   Prereleases use STRICT 2-digit numbering (01-99) for proper alphabetical sorting:
   v1.2.3-prerelease01, v1.2.3-prerelease02, ..., v1.2.3-prerelease99
   
   LEGACY SUPPORT: Historical 1-digit prerelease tags are ignored unless they are
   the latest prerelease for a specific version.

SAFETY FEATURES:
   - Prevents creating versions that already exist for any module
   - Verifies builds and runs tests before creating tags
   - Requires clean git working directory
   - Enforces releases only from master branch

COMMANDS:
   {{range .Commands}}{{if not .HideHelp}}   {{join .Names ", "}}{{ "\t"}}{{.Usage}}{{ "\n" }}{{end}}{{end}}
`,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := cliApp.RunContext(ctx, os.Args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runRelease(c *cli.Context) error {
	// Parse and validate CLI arguments first
	cfg := &release.Config{
		ExcludedDirs:   []string{"cmd", "internal/tools", "idls"},
		RequiredBranch: "master",
		Version:        c.String("set-version"),
		VersionType:    c.String("type"),
		Prerelease:     c.Bool("prerelease"),
		Verbose:        c.Bool("verbose"),
	}

	if cfg.Version != "" && cfg.VersionType != "" {
		return cli.Exit("Cannot specify both --set-version and --type", 1)
	}

	validTypes := map[string]bool{"major": true, "minor": true, "patch": true}
	if cfg.VersionType != "" && !validTypes[cfg.VersionType] {
		return cli.Exit("Version type must be one of: major, minor, patch", 1)
	}

	// If only --prerelease is specified, set special mode
	if cfg.Version == "" && cfg.VersionType == "" && cfg.Prerelease {
		cfg.VersionType = "prerelease-only"
	}

	// Validate version format if provided
	if cfg.Version != "" {
		if _, err := release.NormalizeVersion(cfg.Version); err != nil {
			return cli.Exit(fmt.Sprintf("Invalid version format: %v", err), 1)
		}
	}

	gitClient := git.NewGitClient(cfg.Verbose)

	repo := fs.NewFileSystemClient(cfg.Verbose)

	manager := release.NewReleaseManager(cfg, gitClient, repo, console.NewManager(os.Stdout, os.Stdin))

	// Get repo root and update cfg
	repoRoot, err := gitClient.GetRepoRoot(c.Context)
	if err != nil {
		return cli.Exit(fmt.Sprintf("Failed to get repository root: %v", err), 1)
	}
	cfg.RepoRoot = repoRoot

	// Run the release process
	if err := manager.Run(c.Context); err != nil {
		if c.Context.Err() != nil {
			return nil
		}
		return cli.Exit(err.Error(), 1)
	}

	return nil
}

func showExamples() {
	fmt.Print(`DETAILED EXAMPLES:

Basic Version Bumps:
   release --type patch                        # v1.2.3 → v1.2.4
   release --type minor                        # v1.2.3 → v1.3.0  
   release --type major                        # v1.2.3 → v2.0.0

Prerelease Workflows:
   release --type patch --prerelease           # v1.2.3 → v1.2.4-prerelease01
   release --prerelease                       # v1.2.4-prerelease01 → v1.2.4-prerelease02
   release --prerelease                       # v1.2.4-prerelease02 → v1.2.4-prerelease03
   release --set-version v1.2.4               # v1.2.4-prerelease03 → v1.2.4 (final)

Explicit Versions:
   release --set-version v2.0.0               # Set exactly v2.0.0
   release --set-version v2.0.0 --prerelease  # Set v2.0.0-prerelease01

Testing Changes:
   release --type minor --dry-run              # See what would happen without changes
   release --prerelease --dry-run              # Preview next prerelease

TYPICAL CADENCE WORKFLOW DEVELOPMENT CYCLE:

1. Start new feature development:
   release --type minor --prerelease           # v1.3.0-prerelease01

2. Iterate during development:
   release --prerelease                       # v1.3.0-prerelease02
   release --prerelease                       # v1.3.0-prerelease03
   # ... test Cadence workflows

3. Release candidate:
   release --prerelease                       # v1.3.0-prerelease04
   # ... final testing

4. Production release:
   release --set-version v1.3.0               # v1.3.0 (final)

5. Hotfix if needed:
   release --type patch                       # v1.3.1

`)
}
