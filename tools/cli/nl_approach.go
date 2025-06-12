package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

// Natural language command interface
// Valid formats: ./cadence-cli ask "show me failed workflows" or ./cadence-cli nl "list all domains"
	
func newNaturalLanguageCommand() *cli.Command {
	return &cli.Command{
		Name:    "ask",
		Aliases: []string{"nl"},
		Usage:   "Ask Cadence what to do in natural language",
		Action: func(c *cli.Context) error {
			// Get input from command line arguments
			prompt := strings.Join(c.Args().Slice(), " ")
			
			if prompt == "" {
				return fmt.Errorf("please provide a natural language prompt, e.g.: cadence ask \"show me failed workflows\"")
			}
			
			fmt.Printf("You asked: %s\n", prompt)
			
			// Call Claude directly for translation
			suggested, err := callClaudeCode(prompt)
			if err != nil {
				return fmt.Errorf("failed to get command suggestion: %v", err)
			}
			
			fmt.Printf("Claude suggests: %s\n", suggested)
			
			return nil
		},
	}
}

// Call claude-code command line tool (same logic as MCP server)
func callClaudeCode(prompt string) (string, error) {
	// Prepare the system prompt for Claude
	systemPrompt := `You are a Cadence workflow engine expert. Translate natural language requests into proper Cadence CLI commands. Always respond with only just the command, no explanation or analysis. Translate: ` + prompt

	// Find claude path - check environment variable first, then common locations
	claudePath := findClaudePath()
	if claudePath == "" {
		return "", fmt.Errorf("claude not found. Please install claude or set CLAUDE_PATH environment variable")
	}
	
	cmd := exec.Command(claudePath, "--print", systemPrompt)
	
	// Set the required environment variables for Vertex AI
	// Use environment variables or defaults
	projectID := os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID")
	if projectID == "" {
		return "", fmt.Errorf("ANTHROPIC_VERTEX_PROJECT_ID environment variable not set")
	}
	
	region := os.Getenv("CLOUD_ML_REGION")
	if region == "" {
		region = "us-east5" // default region
	}
	
	cmd.Env = append(os.Environ(),
		"ANTHROPIC_VERTEX_PROJECT_ID="+projectID,
		"CLAUDE_CODE_USE_VERTEX=1",
		"CLOUD_ML_REGION="+region,
	)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to call claude: %v", err)
	}
	
	result := strings.TrimSpace(string(output))
	if result != "" {
		return result, nil
	}

	return "", fmt.Errorf("empty response from claude")
}

// findClaudePath attempts to locate the claude binary
func findClaudePath() string {
	// 1. Check environment variable first
	if claudePath := os.Getenv("CLAUDE_PATH"); claudePath != "" {
		if _, err := os.Stat(claudePath); err == nil {
			return claudePath
		}
	}
	
	// 2. Check if claude is in PATH
	if claudePath, err := exec.LookPath("claude"); err == nil {
		return claudePath
	}
	
	// 3. Check common installation locations
	commonPaths := []string{
		"/usr/local/bin/claude",
		"/opt/homebrew/bin/claude",
		os.ExpandEnv("$HOME/.local/bin/claude"),
		os.ExpandEnv("$HOME/.nvm/versions/node/*/bin/claude"),
	}
	
	for _, path := range commonPaths {
		// Handle glob pattern for nvm
		if strings.Contains(path, "*") {
			matches, _ := filepath.Glob(path)
			for _, match := range matches {
				if _, err := os.Stat(match); err == nil {
					return match
				}
			}
		} else {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	
	return ""
}
