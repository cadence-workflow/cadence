package dependencyage

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	run("init", "-q", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module m\n\nrequire a.com/x v1.0.0\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "go.mod"),
		[]byte("module m/sub\n"), 0o644))
	run("add", ".")
	run("commit", "-q", "-m", "base")
	run("checkout", "-q", "-b", "feature")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module m\n\nrequire a.com/x v1.1.0\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.txt"), []byte("x"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "newpkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "newpkg", "go.mod"),
		[]byte("module m/newpkg\n\nrequire b.com/y/v2 v2.0.0\n"), 0o644))
	run("add", ".")
	run("commit", "-q", "-m", "head")
	return dir
}

func TestGitSource(t *testing.T) {
	dir := initTestRepo(t)
	s := &GitSource{Dir: dir}

	files, err := s.ChangedGoModFiles("main")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"go.mod", "newpkg/go.mod"}, files)

	head, base, baseExists, err := s.Contents("main", "go.mod")
	require.NoError(t, err)
	assert.True(t, baseExists)
	assert.Contains(t, head, "v1.1.0")
	assert.Contains(t, base, "v1.0.0")

	_, _, baseExists, err = s.Contents("main", "newpkg/go.mod")
	require.NoError(t, err)
	assert.False(t, baseExists)

	_, err = s.ChangedGoModFiles("no-such-ref")
	require.Error(t, err)
}

func TestGitSourceUsesMergeBase(t *testing.T) {
	dir := initTestRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	// Advance main past the merge-base with its own go.mod change.
	run("checkout", "-q", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module m\n\nrequire a.com/x v9.9.9\n"), 0o644))
	run("commit", "-q", "-am", "main moved")
	run("checkout", "-q", "feature")

	s := &GitSource{Dir: dir}

	// File list must still reflect the feature branch's own changes only.
	files, err := s.ChangedGoModFiles("main")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"go.mod", "newpkg/go.mod"}, files)

	// Base content must come from the merge-base (v1.0.0), not main's tip (v9.9.9).
	head, base, baseExists, err := s.Contents("main", "go.mod")
	require.NoError(t, err)
	assert.True(t, baseExists)
	assert.Contains(t, head, "v1.1.0")
	assert.Contains(t, base, "v1.0.0")
	assert.NotContains(t, base, "v9.9.9")
}
