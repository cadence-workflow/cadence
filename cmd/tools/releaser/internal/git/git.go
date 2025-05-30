package git

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// Client implements Interface
type Client struct {
	logger *zap.Logger
}

func NewGitClient(logger *zap.Logger) *Client {
	return &Client{logger: logger}
}

func (g *Client) GetCurrentBranch(ctx context.Context) (string, error) {
	g.logger.Debug("Getting current git branch")
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get current branch: %w", err)
	}
	branch := strings.TrimSpace(string(output))
	g.logger.Debug("Current branch", zap.String("branch", branch))
	return branch, nil
}

func (g *Client) GetTags(ctx context.Context) ([]string, error) {
	g.logger.Debug("Fetching git tags")
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
	g.logger.Debug("Found git tags", zap.Int("count", len(tags)))
	return tags, scanner.Err()
}

func (g *Client) CreateTag(ctx context.Context, tag string) error {
	g.logger.Info("Creating git tag", zap.String("tag", tag))
	cmd := exec.CommandContext(ctx, "git", "tag", tag)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("create tag %s: %w", tag, err)
	}
	return err
}

func (g *Client) PushTag(ctx context.Context, tag string) error {
	g.logger.Info("Pushing git tag", zap.String("tag", tag))
	cmd := exec.CommandContext(ctx, "git", "push", "origin", tag)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("push tag %s: %w", tag, err)
	}
	return err
}

func (g *Client) GetRepoRoot(ctx context.Context) (string, error) {
	g.logger.Debug("Getting repository root")
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get repository root: %w", err)
	}
	root := strings.TrimSpace(string(output))
	g.logger.Debug("Repository root", zap.String("root", root))
	return root, nil
}
