package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const mainModule = "github.com/uber/cadence"

func generateTags(version string, goListOutput string, root string) (string, error) {
	tags := []string{version}

	for _, line := range strings.Split(goListOutput, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			continue
		}
		modulePath, moduleDir := parts[0], parts[1]

		if strings.HasPrefix(modulePath, mainModule+"/") {
			relPath, err := filepath.Rel(root, moduleDir)
			if err != nil {
				return "", fmt.Errorf("error getting relative path for %s: %v", moduleDir, err)
			}
			tags = append(tags, fmt.Sprintf("%s/%s", relPath, version))
		}
	}

	if len(tags) == 0 {
		return "", nil
	}
	tagsStr := strings.Join(tags, " ")
	return fmt.Sprintf("git tag %s\ngit push origin %s\n", tagsStr, tagsStr), nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <version>")
		os.Exit(1)
	}
	version := os.Args[1]

	cmd := exec.Command("go", "list", "-m", "-f", "{{.Path}}\t{{.Dir}}", "all")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error running 'go list': %v\n", err)
		os.Exit(1)
	}

	root, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	output, err := generateTags(version, out.String(), root)
	if err != nil {
		fmt.Printf("Error generating tags: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(output)
}
