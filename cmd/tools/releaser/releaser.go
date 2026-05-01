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

func main() {
	cliApp := &cli.App{
		Name:    "releaser",
		Usage:   "Cadence workflow release management tool",
		Version: "0.1.0",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"i"},
				Usage:   "Enable verbose output",
			},
			&cli.BoolFlag{
				Name:  "yes",
				Usage: "Skip confirmation prompts",
			},
			&cli.StringFlag{
				Name:    "set-version",
				Aliases: []string{"s"},
				Usage:   "Override automatic version calculation with specific version",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "release",
				Usage:  "Promote latest prerelease to final release",
				Action: releaseCommand,
				Description: `Converts the current prerelease version to a final release.
Example: v1.2.3-prerelease01 → v1.2.3

This command requires that the current version is a prerelease.`,
			},
			{
				Name:   "minor",
				Usage:  "Start new minor version development cycle",
				Action: minorCommand,
				Description: `Increments the minor version and creates the first prerelease.
Example: v1.2.3 → v1.3.0-prerelease01

Use this when starting development of new features.`,
			},
			{
				Name:   "major",
				Usage:  "Start new major version development cycle",
				Action: majorCommand,
				Description: `Increments the major version and creates the first prerelease.
Example: v1.2.3 → v2.0.0-prerelease01

Use this when introducing breaking changes.`,
			},
			{
				Name:   "patch",
				Usage:  "Start new patch version development cycle",
				Action: patchCommand,
				Description: `Increments the patch version and creates the first prerelease.
Example: v1.2.3 → v1.2.4-prerelease01

Use this when starting hotfix or patch development.`,
			},
			{
				Name:   "prerelease",
				Usage:  "Increment prerelease number",
				Action: prereleaseCommand,
				Description: `Increments the prerelease number for iterative development.
Example: v1.2.3-prerelease01 → v1.2.3-prerelease02

Use this during active development within a version cycle.`,
			},
			{
				Name:   "status",
				Usage:  "Show current repository release status",
				Action: statusCommand,
				Description: `Displays current branch, version, and module information.
Use this to understand the current state before making releases.`,
			},
		},
		CustomAppHelpTemplate: `NAME:
   {{.Name}} - {{.Usage}}

USAGE:
   {{.HelpName}} [global options] command [command options]

VERSION:
   {{.Version}}

GLOBAL OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}

COMMANDS:
{{range .Commands}}{{if not .HideHelp}}   {{join .Names ", "}}{{ "\t\t"}}{{.Usage}}{{ "\n" }}{{end}}{{end}}

EXAMPLES:
   # Check current status
   {{.HelpName}} status

   # Development workflow
   {{.HelpName}} minor           # Start new minor version: v1.2.3 → v1.3.0-prerelease01
   {{.HelpName}} major           # Start new major version: v1.2.3 → v2.0.0-prerelease01
   {{.HelpName}} patch           # Start new patch version: v1.2.3 → v1.2.4-prerelease01
   {{.HelpName}} prerelease      # Iterate: v1.3.0-prerelease01 → v1.3.0-prerelease02
   {{.HelpName}} release         # Finalize: v1.3.0-prerelease03 → v1.3.0

   # Major version workflow  
   {{.HelpName}} major           # Start major version: v1.3.0 → v2.0.0-prerelease01
   {{.HelpName}} prerelease      # Iterate: v2.0.0-prerelease01 → v2.0.0-prerelease02
   {{.HelpName}} release         # Finalize: v2.0.0-prerelease02 → v2.0.0

   # Manual version override
   {{.HelpName}} release --set-version v1.4.0    # Override automatic calculation

SAFETY FEATURES:
   - Validates current version state before operations
   - Requires clean git working directory
   - Enforces releases only from master branch
   - Prevents creating duplicate versions
   - Builds and tests before creating tags
   - Interactive confirmations for all operations

Use --yes to skip confirmation prompts for automation.
`,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := cliApp.RunContext(ctx, os.Args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func createManager(c *cli.Context, command string) (*release.Manager, error) {
	cfg := release.Config{
		ExcludedDirs:      []string{"cmd", "internal/tools", "idls"},
		RequiredBranch:    "master",
		Verbose:           c.Bool("verbose"),
		Command:           command,
		SkipConfirmations: c.Bool("yes"),
		ManualVersion:     c.String("set-version"),
	}

	gitClient := git.NewGitClient(cfg.Verbose)
	repo := fs.NewFileSystemClient(cfg.Verbose)
	interaction := console.NewManager(os.Stdout, os.Stdin)

	manager := release.NewReleaseManager(cfg, gitClient, repo, interaction)

	// Get repo root and update cfg
	repoRoot, err := gitClient.GetRepoRoot(c.Context)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository root: %w", err)
	}
	cfg.RepoRoot = repoRoot

	return manager, nil
}

func releaseCommand(c *cli.Context) error {
	manager, err := createManager(c, "release")
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}

	if err := manager.RunRelease(c.Context); err != nil {
		if c.Context.Err() != nil {
			return nil
		}
		return cli.Exit(err.Error(), 1)
	}

	return nil
}

func minorCommand(c *cli.Context) error {
	manager, err := createManager(c, "minor")
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}

	if err := manager.RunMinor(c.Context); err != nil {
		if c.Context.Err() != nil {
			return nil
		}
		return cli.Exit(err.Error(), 1)
	}

	return nil
}

func majorCommand(c *cli.Context) error {
	manager, err := createManager(c, "major")
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}

	if err := manager.RunMajor(c.Context); err != nil {
		if c.Context.Err() != nil {
			return nil
		}
		return cli.Exit(err.Error(), 1)
	}

	return nil
}

func patchCommand(c *cli.Context) error {
	manager, err := createManager(c, "patch")
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}

	if err := manager.RunPatch(c.Context); err != nil {
		if c.Context.Err() != nil {
			return nil
		}
		return cli.Exit(err.Error(), 1)
	}

	return nil
}

func prereleaseCommand(c *cli.Context) error {
	manager, err := createManager(c, "prerelease")
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}

	if err := manager.RunPrerelease(c.Context); err != nil {
		if c.Context.Err() != nil {
			return nil
		}
		return cli.Exit(err.Error(), 1)
	}

	return nil
}

func statusCommand(c *cli.Context) error {
	manager, err := createManager(c, "status")
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}

	if err := manager.ShowCurrentState(c.Context); err != nil {
		if c.Context.Err() != nil {
			return nil
		}
		return cli.Exit(err.Error(), 1)
	}

	return nil
}
