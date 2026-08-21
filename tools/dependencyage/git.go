package dependencyage

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitSource reads changed go.mod files and their base and head contents using
// the git CLI.
type GitSource struct {
	Dir          string
	mergeBaseRef string
	mergeBaseSHA string
}

// ChangedGoModFiles returns added, copied, modified, and renamed go.mod files
// between baseRef and HEAD.
func (s *GitSource) ChangedGoModFiles(baseRef string) ([]string, error) {
	mergeBaseSHA, err := s.mergeBase(baseRef)
	if err != nil {
		return nil, err
	}
	output, err := s.git(
		"diff",
		"--name-only",
		"--diff-filter=ACMR",
		mergeBaseSHA+"..HEAD",
		"--",
		"go.mod",
		"**/go.mod",
	)
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	var files []string
	for _, path := range strings.Split(string(output), "\n") {
		if path != "" {
			files = append(files, path)
		}
	}
	return files, nil
}

// Contents returns the working-tree and base-ref contents for path. A failed
// git show means the path did not exist at the base ref.
func (s *GitSource) Contents(
	baseRef string,
	path string,
) (head string, base string, baseExists bool, err error) {
	headContent, err := os.ReadFile(filepath.Join(s.directory(), filepath.FromSlash(path)))
	if err != nil {
		return "", "", false, fmt.Errorf("read working-tree file %s: %w", path, err)
	}

	mergeBaseSHA, err := s.mergeBase(baseRef)
	if err != nil {
		return "", "", false, err
	}
	baseContent, err := s.git("show", mergeBaseSHA+":"+path)
	if err != nil {
		return string(headContent), "", false, nil
	}
	return string(headContent), string(baseContent), true, nil
}

func (s *GitSource) mergeBase(baseRef string) (string, error) {
	if s.mergeBaseRef == baseRef && s.mergeBaseSHA != "" {
		return s.mergeBaseSHA, nil
	}

	output, err := s.git("merge-base", baseRef, "HEAD")
	if err != nil {
		return "", fmt.Errorf("git merge-base failed: %w", err)
	}
	mergeBaseSHA := strings.TrimSpace(string(output))
	if mergeBaseSHA == "" {
		return "", fmt.Errorf("git merge-base failed: empty output")
	}
	s.mergeBaseRef = baseRef
	s.mergeBaseSHA = mergeBaseSHA
	return s.mergeBaseSHA, nil
}

func (s *GitSource) git(args ...string) ([]byte, error) {
	commandArgs := append([]string{"-C", s.directory()}, args...)
	cmd := exec.Command("git", commandArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s", err, message)
	}
	return output, nil
}

func (s *GitSource) directory() string {
	if s.Dir == "" {
		return "."
	}
	return s.Dir
}
