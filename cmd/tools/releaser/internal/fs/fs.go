package fs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"golang.org/x/mod/modfile"
)

// Client implements Interface
type Client struct {
	logger *zap.Logger
}

func NewFileSystemClient(logger *zap.Logger) *Client {
	return &Client{logger: logger}
}

// FindGoModFiles reads go.work file and returns module directories
func (f *Client) FindGoModFiles(ctx context.Context, root string) ([]string, error) {
	f.logger.Debug("Finding modules from go.work file", zap.String("root", root))

	workFilePath := filepath.Join(root, "go.work")
	modules, err := f.parseGoWorkFile(workFilePath, root)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.work file: %w", err)
	}

	f.logger.Debug("Found modules from go.work", zap.Int("count", len(modules)))
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

	var modules []string
	for _, use := range workFile.Use {
		absPath := f.resolveModulePath(use.Path, root)
		modules = append(modules, absPath)
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
