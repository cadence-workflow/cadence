package fs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
)

// Client implements Interface
type Client struct {
	verbose bool
}

func NewFileSystemClient(verbose bool) *Client {
	return &Client{verbose: verbose}
}

// FindGoModFiles reads go.work file and returns module directories
func (f *Client) FindGoModFiles(ctx context.Context, root string) ([]string, error) {
	f.logDebug("Finding modules from go.work file")

	workFilePath := filepath.Join(root, "go.work")
	modules, err := f.parseGoWorkFile(workFilePath, root)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.work file: %w", err)
	}

	f.logDebug("Found modules from go.work: %v", modules)
	return modules, nil
}

// parseGoWorkFile parses the go.work file using the official modfile package
func (f *Client) parseGoWorkFile(workFilePath, root string) ([]string, error) {
	workFileData, err := os.ReadFile(workFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.work file: %w", err)
	}

	workFile, err := modfile.ParseWork(workFilePath, workFileData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.work file: %w", err)
	}

	modules := make([]string, 0, len(workFile.Use))
	for _, use := range workFile.Use {
		modules = append(modules, use.Path)
	}

	return modules, nil
}

// resolveModulePath converts relative path to absolute path
func (f *Client) resolveModulePath(modulePath, root string) string {
	if filepath.IsAbs(modulePath) {
		return modulePath
	}

	return filepath.Join(root, modulePath)
}

func (f *Client) logDebug(msg string, args ...interface{}) {
	if f.verbose {
		fmt.Printf("%s\n", fmt.Sprintf(msg, args...))
	}
}
