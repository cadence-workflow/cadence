package fs

import (
	"os"
	"os/exec"
	"path/filepath"

	"go.uber.org/zap"
)

// Client implements Interface
type Client struct {
	logger *zap.Logger
}

func NewFileSystemClient(logger *zap.Logger) *Client {
	return &Client{logger: logger}
}

func (f *Client) FindGoModFiles(root string) ([]string, error) {
	f.logger.Debug("Finding go.mod files", zap.String("root", root))
	var goModFiles []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Name() == "go.mod" {
			goModFiles = append(goModFiles, filepath.Dir(path))
		}
		return nil
	})

	f.logger.Debug("Found go.mod files", zap.Int("count", len(goModFiles)))
	return goModFiles, err
}

func (f *Client) ModTidy(dir string) error {
	f.logger.Debug("Running go mod tidy", zap.String("dir", dir))
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil
	err := cmd.Run()
	if err != nil {
		f.logger.Error("go mod tidy failed", zap.String("dir", dir), zap.Error(err))
	}
	return err
}

func (f *Client) Build(dir string) error {
	f.logger.Debug("Building Go module", zap.String("dir", dir))
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil
	err := cmd.Run()
	if err != nil {
		f.logger.Error("go build failed", zap.String("dir", dir), zap.Error(err))
	}
	return err
}
