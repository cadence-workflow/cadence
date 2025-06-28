package git

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Client implements Interface
type Client struct {
	verbose bool
}

func NewGitClient(verbose bool) *Client {
	return &Client{verbose: verbose}
}

func (g *Client) GetCurrentBranch(ctx context.Context) (string, error) {
	g.logDebug("Getting current git branch")
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get current branch: %w", err)
	}
	branch := strings.TrimSpace(string(output))
	g.logDebug("Current branch: %s", branch)
	return branch, nil
}

func (g *Client) GetTags(ctx context.Context) ([]string, error) {
	g.logDebug("Fetching git tags")
	cmd := exec.CommandContext(ctx, "git", "tag", "-l")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get git tags: %w", err)
	}

	var tags []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		if tag := strings.TrimSpace(scanner.Text()); tag != "" {
			tags = append(tags, tag)
		}
	}
	g.logDebug("Found git tags %v", tags)
	return tags, scanner.Err()
}

func (g *Client) CreateTag(ctx context.Context, tag string) error {
	g.logDebug("Creating git tag %s", tag)
	cmd := exec.CommandContext(ctx, "git", "tag", tag)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("create tag %s: %w", tag, err)
	}
	return err
}

func (g *Client) PushTag(ctx context.Context, tag string) error {
	g.logDebug("Pushing git tag %s", tag)
	cmd := exec.CommandContext(ctx, "git", "push", "origin", tag)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("push tag %s: %w", tag, err)
	}
	return err
}

func (g *Client) GetRepoRoot(ctx context.Context) (string, error) {
	g.logDebug("Getting repository root")
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get repository root: %w", err)
	}
	root := strings.TrimSpace(string(output))
	g.logDebug("Repository root %s", root)
	return root, nil
}

func (g *Client) logDebug(msg string, args ...interface{}) {
	if g.verbose {
		fmt.Printf("%s\n", fmt.Sprintf(msg, args...))
	}
}
