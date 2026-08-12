package common

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/uber/cadence/common/persistence"
)

const (
	VersionsPath = "versioned"
	ManifestPath = "manifest.json"
	newLineDelim = '\n'
)

type (
	embeddedSchema struct {
		latest              persistence.Version
		Files               fs.FS
		versionsPath        string
		skipToLatestDDLPath string
	}

	// manifest is a value type that represents
	// the deserialized manifest.json file within
	// a schema version directory
	manifest struct {
		CurrVersion          string
		MinCompatibleVersion string
		Description          string
		SchemaUpdateCqlFiles []string
		MD5                  string
	}
)

func EmbeddedSchema(files fs.FS, version string, path string, skipToLatestDDLPath string) persistence.Schema {
	return &embeddedSchema{
		latest:              mustParseVersion(version),
		versionsPath:        filepath.Join(path, VersionsPath),
		Files:               files,
		skipToLatestDDLPath: filepath.Join(path, skipToLatestDDLPath),
	}
}

func (s *embeddedSchema) LatestVersion() persistence.Version {
	return s.latest
}

func (s *embeddedSchema) AllUpdates() ([]*persistence.SchemaUpdate, error) {
	subdirs, err := fs.ReadDir(s.Files, s.versionsPath)
	if err != nil {
		return nil, err
	}

	var updates []*persistence.SchemaUpdate
	for _, dir := range subdirs {
		// Not supporting embeddedSchema Squashing as added here: https://github.com/cadence-workflow/cadence/pull/3253
		// They're optional and have never been used since
		if dir.IsDir() && strings.HasPrefix(dir.Name(), "v") {
			version := strings.TrimPrefix(dir.Name(), "v")
			update, uErr := parseUpdate(s.Files, filepath.Join(s.versionsPath, dir.Name()), version)
			if uErr != nil {
				return nil, uErr
			}
			updates = append(updates, update)
		}
	}

	slices.SortStableFunc(updates, func(a, b *persistence.SchemaUpdate) int {
		return a.Version.Compare(b.Version)
	})

	return updates, nil
}

func (s *embeddedSchema) SkipToLatest() (*persistence.SchemaUpdate, error) {
	file, err := s.Files.Open(s.skipToLatestDDLPath)
	if err != nil {
		return nil, err
	}
	ddl, err := parseDDL(file)
	if err != nil {
		return nil, err
	}
	return &persistence.SchemaUpdate{
		Version:              s.latest,
		MinCompatibleVersion: s.latest,
		DDLStatements:        ddl,
		ManifestMD5:          "",
		Description:          "Skip to latest schema update",
	}, nil
}

func parseUpdate(root fs.FS, path, version string) (*persistence.SchemaUpdate, error) {
	man, err := readManifest(root, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest for version %s: %w", version, err)
	}
	if man.CurrVersion != version {
		return nil, fmt.Errorf("version %s does not match manifest version: %s", version, man.CurrVersion)
	}

	update := &persistence.SchemaUpdate{
		Description: man.Description,
		ManifestMD5: man.MD5,
	}
	curVer, err := persistence.ParseVersion(man.CurrVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid version: %w", err)
	}
	update.Version = curVer
	minVer, err := persistence.ParseVersion(man.MinCompatibleVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid min version: %w", err)
	}
	update.MinCompatibleVersion = minVer

	ddl, err := readDDLStatements(root, path, man)
	if err != nil {
		return nil, fmt.Errorf("failed to read ddl statements: %w", err)
	}
	update.DDLStatements = ddl

	return update, nil
}

func readManifest(fileSystem fs.FS, subdir string) (*manifest, error) {
	fsys, err := fs.Sub(fileSystem, subdir)
	if err != nil {
		return nil, err
	}
	file, err := fsys.Open(ManifestPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	jsonBlob, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var m manifest
	err = json.Unmarshal(jsonBlob, &m)
	if err != nil {
		return nil, err
	}

	// Only for metadata, not a secure usage
	// #nosec
	md5Bytes := md5.Sum(jsonBlob)
	m.MD5 = hex.EncodeToString(md5Bytes[:])

	return &m, nil
}

func readDDLStatements(root fs.FS, subdir string, man *manifest) ([]string, error) {
	var result []string

	for _, file := range man.SchemaUpdateCqlFiles {
		path := filepath.Join(subdir, file)
		f, err := root.Open(path)
		if err != nil {
			return nil, fmt.Errorf("error opening file %v, err=%v", path, err)
		}
		stmts, err := parseDDL(f)
		if err != nil {
			return nil, fmt.Errorf("error parsing file %v, err=%v", path, err)
		}
		result = append(result, stmts...)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("found 0 updates in dir: %v", subdir)
	}

	return result, nil
}

func parseDDL(file fs.File) ([]string, error) {
	reader := bufio.NewReader(file)

	var line string
	var currStmt string
	var stmts []string
	var err error

	for err == nil {

		line, err = reader.ReadString(newLineDelim)
		line = strings.TrimSpace(line)
		if len(line) < 1 {
			continue
		}

		// Filter out the comment lines, the
		// only recognized comment line format
		// is any line that starts with double dashes
		tokens := strings.Split(line, "--")
		if len(tokens) > 0 && len(tokens[0]) > 0 {
			currStmt += tokens[0]
			// semi-colon is the end of statement delim
			if strings.HasSuffix(currStmt, ";") {
				stmts = append(stmts, currStmt)
				currStmt = ""
			}
		}
	}

	if err == io.EOF {
		return stmts, nil
	}

	return nil, err
}

func mustParseVersion(v string) persistence.Version {
	version, err := persistence.ParseVersion(v)
	if err != nil {
		panic(err)
	}
	return version
}
