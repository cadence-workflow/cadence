// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/schema/cassandra"
	"github.com/uber/cadence/schema/mysql"
	"github.com/uber/cadence/schema/postgres"
	"github.com/uber/cadence/schema/sqlite"
)

type UpdateTaskTestSuite struct {
	*require.Assertions // override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test, not merely log an error
	suite.Suite
}

func TestUpdateTaskTestSuite(t *testing.T) {
	suite.Run(t, new(UpdateTaskTestSuite))
}

func (s *UpdateTaskTestSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *UpdateTaskTestSuite) TestReadSchemaDir() {

	emptyDir := s.T().TempDir()
	tmpDir := s.T().TempDir()
	squashDir := s.T().TempDir()

	subDirs := []string{"v0.5", "v1.5", "v2.5", "v3.5", "v10.2", "abc", "2.0", "3.0"}
	for _, d := range subDirs {
		s.NoError(os.Mkdir(tmpDir+"/"+d, os.FileMode(0444)))
	}

	squashDirs := []string{"v0.5", "v1.5", "v2.5", "v3.5", "v10.2", "abc", "2.0", "3.0", "s0.5-10.2", "s1.5-3.5"}
	for _, d := range squashDirs {
		s.NoError(os.Mkdir(squashDir+"/"+d, os.FileMode(0444)))
	}

	_, err := readSchemaDir(os.DirFS(tmpDir), "11.0", "11.2")
	s.Error(err)
	_, err = readSchemaDir(os.DirFS(tmpDir), "0.5", "10.3")
	s.Error(err)
	_, err = readSchemaDir(os.DirFS(tmpDir), "1.5", "1.5")
	s.Error(err)
	_, err = readSchemaDir(os.DirFS(tmpDir), "1.5", "0.5")
	s.Error(err)
	_, err = readSchemaDir(os.DirFS(tmpDir), "10.3", "")
	s.Error(err)
	_, err = readSchemaDir(os.DirFS(emptyDir), "11.0", "")
	s.Error(err)
	_, err = readSchemaDir(os.DirFS(emptyDir), "10.1", "")
	s.Error(err)

	ans, err := readSchemaDir(os.DirFS(tmpDir), "0.4", "10.2")
	s.NoError(err)
	s.Equal([]string{"v0.5", "v1.5", "v2.5", "v3.5", "v10.2"}, ans)

	ans, err = readSchemaDir(os.DirFS(tmpDir), "0.5", "3.5")
	s.NoError(err)
	s.Equal([]string{"v1.5", "v2.5", "v3.5"}, ans)

	ans, err = readSchemaDir(os.DirFS(tmpDir), "10.2", "")
	s.NoError(err)
	s.Equal(0, len(ans))

	ans, err = readSchemaDir(os.DirFS(squashDir), "0.4", "10.2")
	s.NoError(err)
	s.Equal([]string{"v0.5", "s0.5-10.2"}, ans)

	ans, err = readSchemaDir(os.DirFS(squashDir), "0.5", "3.5")
	s.NoError(err)
	s.Equal([]string{"v1.5", "s1.5-3.5"}, ans)

	ans, err = readSchemaDir(os.DirFS(squashDir), "10.2", "")
	s.NoError(err)
	s.Empty(ans)
}

func (s *UpdateTaskTestSuite) TestReadSchemaDirFromEmbeddings() {
	// Cassandra
	fsys, err := fs.Sub(cassandra.SchemaFS, "cadence/versioned")
	s.NoError(err)
	ans, err := readSchemaDir(fsys, "0.30", "")
	s.NoError(err)
	s.Equal([]string{"v0.31", "v0.32", "v0.33", "v0.34", "v0.35", "v0.36", "v0.37", "v0.38", "v0.39", "v0.40", "v0.41", "v0.42", "v0.43", "v0.44", "v0.45"}, ans)

	fsys, err = fs.Sub(cassandra.SchemaFS, "visibility/versioned")
	s.NoError(err)
	ans, err = readSchemaDir(fsys, "0.6", "")
	s.NoError(err)
	s.Equal([]string{"v0.7", "v0.8", "v0.9", "v0.10"}, ans)

	// MySQL
	fsys, err = fs.Sub(mysql.SchemaFS, "v8/cadence/versioned")
	s.NoError(err)
	ans, err = readSchemaDir(fsys, "0.3", "")
	s.NoError(err)
	s.Equal([]string{"v0.4", "v0.5", "v0.6"}, ans)

	fsys, err = fs.Sub(mysql.SchemaFS, "v8/visibility/versioned")
	s.NoError(err)
	ans, err = readSchemaDir(fsys, "0.5", "")
	s.NoError(err)
	s.Equal([]string{"v0.6", "v0.7", "v0.8"}, ans)

	// SQLite
	fsys, err = fs.Sub(sqlite.SchemaFS, "cadence/versioned")
	s.NoError(err)
	ans, err = readSchemaDir(fsys, "0.1", "")
	s.NoError(err)
	s.Nil(ans, "no version dirs found after 0.1")

	fsys, err = fs.Sub(sqlite.SchemaFS, "visibility/versioned")
	s.NoError(err)
	ans, err = readSchemaDir(fsys, "0.1", "")
	s.NoError(err)
	s.Equal([]string{"v0.2"}, ans)

	// Postgres
	fsys, err = fs.Sub(postgres.SchemaFS, "cadence/versioned")
	s.NoError(err)
	ans, err = readSchemaDir(fsys, "0.3", "")
	s.NoError(err)
	s.Equal([]string{"v0.4", "v0.5", "v0.6", "v0.7"}, ans)

	fsys, err = fs.Sub(postgres.SchemaFS, "visibility/versioned")
	s.NoError(err)
	ans, err = readSchemaDir(fsys, "0.5", "")
	s.NoError(err)
	s.Equal([]string{"v0.6", "v0.7", "v0.8", "v0.9"}, ans)
}

func (s *UpdateTaskTestSuite) TestReadManifest() {

	tmpDir := s.T().TempDir()

	input := `{
		"CurrVersion": "0.4",
		"MinCompatibleVersion": "0.1",
		"Description": "base version of schema",
		"SchemaUpdateCqlFiles": ["base1.cql", "base2.cql", "base3.cql"]
	}`
	files := []string{"base1.cql", "base2.cql", "base3.cql"}
	s.runReadManifestTest(tmpDir, input, "0.4", "0.1", "base version of schema", files, false)

	errInputs := []string{
		`{
			"MinCompatibleVersion": "0.1",
			"Description": "base",
			"SchemaUpdateCqlFiles": ["base1.cql"]
		 }`,
		`{
			"CurrVersion": "0.4",
			"Description": "base version of schema",
			"SchemaUpdateCqlFiles": ["base1.cql", "base2.cql", "base3.cql"]
		 }`,
		`{
			"CurrVersion": "0.4",
			"MinCompatibleVersion": "0.1",
			"Description": "base version of schema",
		 }`,
		`{
			"CurrVersion": "",
			"MinCompatibleVersion": "0.1",
			"Description": "base version of schema",
			"SchemaUpdateCqlFiles": ["base1.cql", "base2.cql", "base3.cql"]
		 }`,
		`{
			"CurrVersion": "0.4",
			"MinCompatibleVersion": "",
			"Description": "base version of schema",
			"SchemaUpdateCqlFiles": ["base1.cql", "base2.cql", "base3.cql"]
		 }`,
		`{
			"CurrVersion": "",
			"MinCompatibleVersion": "0.1",
			"Description": "base version of schema",
			"SchemaUpdateCqlFiles": []
		 }`,
	}

	for _, in := range errInputs {
		s.runReadManifestTest(tmpDir, in, "", "", "", nil, true)
	}
}

func (s *UpdateTaskTestSuite) TestFilterDirectories() {
	tests := []struct {
		name       string
		startVer   string
		endVer     string
		wantErr    string
		emptyDir   bool
		wantResult []string
	}{
		{name: "endVer highter", startVer: "11.0", endVer: "11.2", wantErr: "version dir not found for target version 11.2"},
		{name: "both outside", startVer: "0.5", endVer: "10.3", wantErr: "version dir not found for target version 10.3"},
		{name: "versions equal", startVer: "1.5", endVer: "1.5", wantErr: "startVer (1.5) must be less than endVer (1.5)"},
		{name: "endVer < startVer", startVer: "1.5", endVer: "0.5", wantErr: "startVer (1.5) must be less than endVer (0.5)"},
		{name: "startVer highter", startVer: "10.3", wantErr: "no subdirs found with version >= 10.3"},
		{name: "empty set 1", startVer: "11.0", wantErr: "no subdirs found with version >= 11.0", emptyDir: true},
		{name: "empty set 2", startVer: "10.1", wantErr: "no subdirs found with version >= 10.1", emptyDir: true},
		{name: "all versions", startVer: "0.4", endVer: "10.2", wantResult: []string{"v0.5", "v1.5", "v2.5", "v3.5", "v10.2"}},
		{name: "subset", startVer: "0.5", endVer: "3.5", wantResult: []string{"v1.5", "v2.5", "v3.5"}},
		{name: "already at last version", startVer: "10.2"},
	}
	subDirs := []string{"v0.5", "v1.5", "v2.5", "v3.5", "v10.2", "abc", "2.0", "3.0"}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			var dirs []string
			if !tt.emptyDir {
				dirs = subDirs
			}
			r, sq, err := filterDirectories(dirs, tt.startVer, tt.endVer)
			s.Empty(sq)
			if tt.wantErr != "" {
				s.Empty(r)
				s.EqualError(err, tt.wantErr)
			} else {
				s.NoError(err)
				s.Equal(tt.wantResult, r)
			}
		})
	}
}

func (s *UpdateTaskTestSuite) TestFilterSquashDirectories() {
	tests := []struct {
		name       string
		startVer   string
		endVer     string
		wantErr    string
		wantResult []string
		wantSq     []squashVersion
		customDirs []string
	}{
		{name: "endVer highter", startVer: "11.0", endVer: "11.2", wantErr: "version dir not found for target version 11.2"},
		{name: "both outside", startVer: "0.5", endVer: "10.3", wantErr: "version dir not found for target version 10.3"},
		{name: "versions equal", startVer: "1.5", endVer: "1.5", wantErr: "startVer (1.5) must be less than endVer (1.5)"},
		{name: "endVer < startVer", startVer: "1.5", endVer: "0.5", wantErr: "startVer (1.5) must be less than endVer (0.5)"},
		{name: "startVer highter", startVer: "10.3", wantErr: "no subdirs found with version >= 10.3"},
		{
			name:       "backward version squash",
			startVer:   "1.5",
			wantErr:    "invalid squashed version \"s3.0-2.0\", 3.0 >= 2.0",
			customDirs: []string{"v0.5", "v1.5", "v2.5", "v3.5", "v10.2", "abc", "2.0", "3.0", "s0.5-10.2", "s1.5-3.5", "s3.0-2.0"},
		},
		{
			name:       "equal version squash",
			startVer:   "1.5",
			wantErr:    "invalid squashed version \"s2.0-2.0\", 2.0 >= 2.0",
			customDirs: []string{"v0.5", "v1.5", "v2.5", "v3.5", "v10.2", "abc", "2.0", "3.0", "s0.5-10.2", "s1.5-3.5", "s2.0-2.0"},
		},
		{
			name:       "all versions",
			startVer:   "0.4",
			endVer:     "10.2",
			wantResult: []string{"v0.5", "v1.5", "v2.5", "v3.5", "v10.2"},
			wantSq: []squashVersion{
				{prev: "1.5", ver: "3.5", dirName: "s1.5-3.5"},
				{prev: "0.5", ver: "10.2", dirName: "s0.5-10.2"},
			},
		},
		{
			name:       "subset",
			startVer:   "0.5",
			endVer:     "3.5",
			wantResult: []string{"v1.5", "v2.5", "v3.5"},
			wantSq: []squashVersion{
				{prev: "1.5", ver: "3.5", dirName: "s1.5-3.5"},
			},
		},
		{name: "already at last version", startVer: "10.2"},
	}
	subDirs := []string{"v0.5", "v1.5", "v2.5", "v3.5", "v10.2", "abc", "2.0", "3.0", "s0.5-10.2", "s1.5-3.5"}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			dirs := subDirs
			if len(tt.customDirs) > 0 {
				dirs = tt.customDirs
			}
			r, sq, err := filterDirectories(dirs, tt.startVer, tt.endVer)
			if tt.wantErr != "" {
				s.Empty(r)
				s.Empty(sq)
				s.EqualError(err, tt.wantErr)
			} else {
				s.NoError(err)
				s.Equal(tt.wantResult, r)
				s.ElementsMatch(tt.wantSq, sq)
			}
		})
	}
}

func (s *UpdateTaskTestSuite) TestSquashDirToVersion() {
	tests := []struct {
		source string
		prev   string
		ver    string
	}{
		{source: "s0.0-0.0", prev: "0.0", ver: "0.0"},
		{source: "s1.0-0.0", prev: "1.0", ver: "0.0"},
		{source: "s1.0-2", prev: "1.0", ver: "2"},
		{source: "s1-2.1", prev: "1", ver: "2.1"},
		{source: "s1-2", prev: "1", ver: "2"},
	}
	for _, tt := range tests {
		s.Run(tt.source, func() {
			prev, ver := squashDirToVersion(tt.source)
			s.Equal(tt.prev, prev)
			s.Equal(tt.ver, ver)
		})
	}
}

func (s *UpdateTaskTestSuite) TestNewUpdateSchemaTask() {
	mockSchemaClient := NewMockSchemaClient(gomock.NewController(s.T()))
	cfg := &UpdateConfig{}

	updateSchemaTask := NewUpdateSchemaTask(mockSchemaClient, cfg)

	s.NotNil(updateSchemaTask)
	s.Equal(mockSchemaClient, updateSchemaTask.db)
	s.Equal(cfg, updateSchemaTask.config)
}

func (s *UpdateTaskTestSuite) TestRun() {
	targetVersion := "0.4"

	tests := map[string]struct {
		setupMock     func(client *MockSchemaClient)
		dryRun        bool
		targetVersion string
		dir           string
		err           error
	}{
		"error - ReadSchemaVersion": {
			setupMock: func(client *MockSchemaClient) {
				client.EXPECT().ReadSchemaVersion().Return("", assert.AnError).Times(1)
			},
			dir: "v0.4",
			err: assert.AnError,
		},
		"error - BuildChangeSet": {
			setupMock: func(client *MockSchemaClient) {
				client.EXPECT().ReadSchemaVersion().Return("1.0", nil).Times(1)
			},
			dir:           "v0.4",
			targetVersion: targetVersion,
			err:           fmt.Errorf("startVer (1.0) must be less than endVer (%v)", targetVersion),
		},
		"error - executeUpdates": {
			setupMock: func(client *MockSchemaClient) {
				client.EXPECT().ReadSchemaVersion().Return("0.3", nil).Times(1)
				client.EXPECT().ExecDDLQuery(gomock.Any()).Return(assert.AnError).Times(1)
			},
			dir:           "v0.4",
			targetVersion: targetVersion,
			err:           assert.AnError,
		},
		"success - dry run": {
			setupMock: func(client *MockSchemaClient) {
				client.EXPECT().ReadSchemaVersion().Return("0.3", nil).Times(1)
			},
			dir:           "v0.4",
			dryRun:        true,
			targetVersion: targetVersion,
			err:           nil,
		},
		"success - dry run no changes squashed version": {
			setupMock: func(client *MockSchemaClient) {
				client.EXPECT().ReadSchemaVersion().Return("0.4", nil).Times(1)
			},
			dir:    "s1-2",
			dryRun: true,
			err:    nil,
		},
		"success": {
			setupMock: func(client *MockSchemaClient) {
				client.EXPECT().ReadSchemaVersion().Return("0.3", nil).Times(1)
				client.EXPECT().ExecDDLQuery(gomock.Any()).Return(nil).Times(1)
				client.EXPECT().UpdateSchemaVersion(targetVersion, targetVersion).Return(nil).Times(1)
				client.EXPECT().WriteSchemaUpdateLog("0.3", targetVersion, gomock.Any(), "").Return(nil).Times(1)
			},
			dir:           "v0.4",
			targetVersion: targetVersion,
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			mockSchemaClient := NewMockSchemaClient(gomock.NewController(s.T()))
			test.setupMock(mockSchemaClient)

			dir := s.T().TempDir()
			subDir := test.dir
			s.NoError(os.Mkdir(filepath.Join(dir, subDir), os.FileMode(0755)))

			manifestFile := filepath.Join(dir, subDir, manifestFileName)
			m := manifest{
				CurrVersion:          targetVersion,
				MinCompatibleVersion: targetVersion,
				SchemaUpdateCqlFiles: []string{"base.cql"},
			}
			j, err := json.Marshal(m)
			s.NoError(err)
			s.NoError(os.WriteFile(manifestFile, j, os.FileMode(0644)))
			cqlFile := filepath.Join(dir, subDir, "base.cql")
			s.NoError(os.WriteFile(cqlFile, []byte("CREATE TABLE;"), os.FileMode(0644)))

			cfg := &UpdateConfig{
				SchemaFS:      os.DirFS(dir),
				TargetVersion: test.targetVersion,
				IsDryRun:      test.dryRun,
			}
			updateSchemaTask := NewUpdateSchemaTask(mockSchemaClient, cfg)

			err = updateSchemaTask.Run()

			if test.err != nil {
				s.ErrorContains(err, test.err.Error())
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *UpdateTaskTestSuite) Test_executeUpdates() {
	tests := map[string]struct {
		setupMock func(client *MockSchemaClient)
		updates   []ChangeSet
		err       error
	}{
		"no updates": {
			setupMock: func(client *MockSchemaClient) {},
			err:       nil,
		},
		"error - updateSchemaVersion": {
			setupMock: func(client *MockSchemaClient) {
				client.EXPECT().UpdateSchemaVersion("0.3", "0.3").Return(assert.AnError).Times(1)
			},
			updates: []ChangeSet{
				{
					Version: "0.3",
					manifest: &manifest{
						CurrVersion:          "0.3",
						MinCompatibleVersion: "0.3",
						SchemaUpdateCqlFiles: []string{"base.cql"},
					},
				},
			},
			err: assert.AnError,
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			mockSchemaClient := NewMockSchemaClient(gomock.NewController(s.T()))
			test.setupMock(mockSchemaClient)

			cfg := &UpdateConfig{}

			updateSchemaTask := NewUpdateSchemaTask(mockSchemaClient, cfg)

			err := updateSchemaTask.executeUpdates("0.3", test.updates)

			if test.err != nil {
				s.ErrorContains(err, test.err.Error())
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *UpdateTaskTestSuite) Test_updateSchemaVersion() {
	version := "0.3"

	tests := map[string]struct {
		setupMock func(client *MockSchemaClient)
		err       error
	}{
		"error - UpdateSchemaVersion": {
			setupMock: func(client *MockSchemaClient) {
				client.EXPECT().UpdateSchemaVersion(version, version).Return(assert.AnError).Times(1)
			},
			err: assert.AnError,
		},
		"error - WriteSchemaVersion": {
			setupMock: func(client *MockSchemaClient) {
				client.EXPECT().UpdateSchemaVersion(version, version).Return(nil).Times(1)
				client.EXPECT().WriteSchemaUpdateLog("0.2", version, "md5", "description").Return(assert.AnError).Times(1)
			},
			err: assert.AnError,
		},
		"success": {
			setupMock: func(client *MockSchemaClient) {
				client.EXPECT().UpdateSchemaVersion(version, version).Return(nil).Times(1)
				client.EXPECT().WriteSchemaUpdateLog("0.2", version, "md5", "description").Return(nil).Times(1)
			},
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			mockSchemaClient := NewMockSchemaClient(gomock.NewController(s.T()))
			test.setupMock(mockSchemaClient)

			cfg := &UpdateConfig{}

			updateSchemaTask := NewUpdateSchemaTask(mockSchemaClient, cfg)

			cs := &ChangeSet{
				Version: version,
				manifest: &manifest{
					CurrVersion:          version,
					MinCompatibleVersion: version,
					md5:                  "md5",
					Description:          "description",
				},
			}

			err := updateSchemaTask.updateSchemaVersion("0.2", cs)

			if test.err != nil {
				s.ErrorContains(err, test.err.Error())
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *UpdateTaskTestSuite) TestBuildChangeSet() {
	tests := map[string]struct {
		manifestFile    string
		updates         []ChangeSet
		manifestVersion string
		currentVersion  string
		cqlFiles        []string
		dirs            []string
		cqlFileContent  string
		err             error
	}{
		"error - readManifest": {
			manifestFile: "wrong_manifest.json",
			dirs:         []string{"v0.4"},
			err:          errors.New("no such file or directory"),
		},
		"error - squash version - manifest current version different from dir version": {
			manifestFile:    manifestFileName,
			dirs:            []string{"s0.3-0.4", "v0.4"},
			currentVersion:  "0.3",
			manifestVersion: "0.3",
			cqlFiles:        []string{"base.cql"},
			err:             errors.New("manifest version doesn't match with dirname, dir=s0.3-0.4,manifest.version=0.3"),
		},
		"error - manifest current version different from dir version": {
			manifestFile:    manifestFileName,
			dirs:            []string{"v0.4"},
			currentVersion:  "0.3",
			manifestVersion: "0.3",
			cqlFiles:        []string{"base.cql"},
			err:             errors.New("manifest version doesn't match with dirname, dir=v0.4,manifest.version=0.3"),
		},
		"error - parseSQLStmts": {
			manifestFile:    manifestFileName,
			dirs:            []string{"v0.4"},
			currentVersion:  "0.3",
			manifestVersion: "0.4",
			cqlFiles:        []string{"base.cql", "wrong.cql"},
			err:             errors.New("error opening file v0.4/wrong.cql, err=open v0.4/wrong.cql: no such file or directory"),
		},
		"error - validateCQLStmts": {
			manifestFile:    manifestFileName,
			dirs:            []string{"v0.4"},
			currentVersion:  "0.3",
			manifestVersion: "0.4",
			cqlFiles:        []string{"base.cql"},
			cqlFileContent:  "WRONG CQL STATEMENT;",
			err:             errors.New("error processing version v0.4:CQL prefix not in whitelist, stmt=WRONG CQL STATEMENT;"),
		},
		"success": {
			manifestFile:    manifestFileName,
			dirs:            []string{"v0.4"},
			currentVersion:  "0.3",
			manifestVersion: "0.4",
			cqlFiles:        []string{"base.cql"},
			cqlFileContent:  "CREATE TABLE;",
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			mockSchemaClient := NewMockSchemaClient(gomock.NewController(s.T()))

			dir := s.T().TempDir()
			for _, subDir := range test.dirs {
				s.NoError(os.Mkdir(filepath.Join(dir, subDir), os.FileMode(0755)))
				manifestFile := filepath.Join(dir, subDir, test.manifestFile)
				m := manifest{
					CurrVersion:          test.manifestVersion,
					MinCompatibleVersion: test.manifestVersion,
					SchemaUpdateCqlFiles: test.cqlFiles,
				}
				j, err := json.Marshal(m)
				s.NoError(err)
				s.NoError(os.WriteFile(manifestFile, j, os.FileMode(0644)))
				cqlFile := filepath.Join(dir, subDir, "base.cql")
				s.NoError(os.WriteFile(cqlFile, []byte(test.cqlFileContent), os.FileMode(0644)))
			}

			cfg := &UpdateConfig{
				SchemaFS:      os.DirFS(dir),
				TargetVersion: "0.4",
			}

			updateSchemaTask := NewUpdateSchemaTask(mockSchemaClient, cfg)

			cs, err := updateSchemaTask.BuildChangeSet(test.currentVersion)

			if test.err != nil {
				s.Nil(cs)
				s.ErrorContains(err, test.err.Error())
			} else {
				s.NoError(err)
				s.NotNil(cs)
			}
		})
	}
}

func (s *UpdateTaskTestSuite) Test_parseSQLStmts() {
	subDir := "v0.4"

	tests := map[string]struct {
		content  string
		cqlFiles []string
		stmts    []string
		err      error
	}{
		"success": {
			stmts:    []string{"CREATE TABLE;", "CREATE TABLE;"},
			cqlFiles: []string{"base.cql"},
			content:  "CREATE TABLE;\n\nCREATE TABLE;\n",
		},
		"error - failed to open file": {
			cqlFiles: []string{"wrong.cql"},
			err:      errors.New("error opening file"),
		},
		"error - no updates": {
			cqlFiles: []string{"base.cql"},
			content:  "",
			err:      fmt.Errorf("found 0 updates in dir %v", subDir),
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			mockSchemaClient := NewMockSchemaClient(gomock.NewController(s.T()))

			dir := s.T().TempDir()

			cfg := &UpdateConfig{
				SchemaFS: os.DirFS(dir),
			}

			s.NoError(os.Mkdir(filepath.Join(dir, subDir), os.FileMode(0755)))

			cqlFile := filepath.Join(dir, subDir, "base.cql")
			s.NoError(os.WriteFile(cqlFile, []byte(test.content), os.FileMode(0644)))

			updateSchemaTask := NewUpdateSchemaTask(mockSchemaClient, cfg)

			m := &manifest{
				SchemaUpdateCqlFiles: test.cqlFiles,
			}

			stmts, err := updateSchemaTask.parseSQLStmts(subDir, m)

			if test.err != nil {
				s.Nil(stmts)
				s.ErrorContains(err, test.err.Error())
			} else {
				s.NoError(err)
				s.Equal(test.stmts, stmts)
			}
		})
	}
}

func (s *UpdateTaskTestSuite) Test_readManifest() {
	tests := map[string]struct {
		subDir        string
		manifestFile  string
		invalidSubDir bool
		cqlFiles      []string
		err           error
	}{
		"error - invalid sub directory name": {
			subDir:        string([]byte{0xff, 0xfe, 0xfd, 0xfc}),
			manifestFile:  manifestFileName,
			invalidSubDir: true,
			err:           fs.ErrInvalid, // bad utf8 returns this as of 1.23: https://github.com/golang/go/commit/bf821f65cfd61dcc431922eea2cb97ce0825d60c
		},
		"error - manifest.json file not found": {
			subDir:       "v0.4",
			manifestFile: "wrong_manifest.json",
			err:          errors.New("no such file or directory"),
		},
		"error - manifest missing SchemaUpdateCqlFiles": {
			subDir:       "v0.4",
			manifestFile: manifestFileName,
			err:          fmt.Errorf("manifest missing SchemaUpdateCqlFiles"),
		},
		"success": {
			subDir:       "v0.4",
			manifestFile: manifestFileName,
			cqlFiles:     []string{"base.cql"},
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			dir := s.T().TempDir()

			var m manifest

			if !test.invalidSubDir {
				subDir := test.subDir

				s.NoError(os.Mkdir(filepath.Join(dir, subDir), os.FileMode(0755)))
				manifestFile := filepath.Join(dir, subDir, test.manifestFile)
				m = manifest{
					CurrVersion:          "0.4",
					MinCompatibleVersion: "0.4",
					Description:          "base version of schema",
					SchemaUpdateCqlFiles: test.cqlFiles,
				}

				j, err := json.Marshal(m)
				s.NoError(err)
				s.NoError(os.WriteFile(manifestFile, j, os.FileMode(0644)))
			}

			m2, err := readManifest(os.DirFS(dir), test.subDir)

			if test.err != nil {
				s.Nil(m2)
				s.ErrorContains(err, test.err.Error())
			} else {
				s.NoError(err)
				s.Equal(m.CurrVersion, m2.CurrVersion)
				s.Equal(m.MinCompatibleVersion, m2.MinCompatibleVersion)
				s.Equal(m.Description, m2.Description)
				s.Equal(m.SchemaUpdateCqlFiles, m2.SchemaUpdateCqlFiles)
				s.Empty(m.md5)
				s.NotEmpty(m2.md5)
			}
		})
	}
}

func (s *UpdateTaskTestSuite) Test_readSchemaDir_ErrorGettingSubDirs() {
	ret, err := readSchemaDir(os.DirFS("testdata"), "0.4", "10.2")
	s.Error(err)
	s.ErrorContains(err, "no such file or directory")
	s.Nil(ret)
}

func (s *UpdateTaskTestSuite) runReadManifestTest(dir, input, currVer, minVer, desc string,
	files []string, isErr bool) {

	file := dir + "/manifest.json"
	err := ioutil.WriteFile(file, []byte(input), os.FileMode(0644))
	s.Nil(err)

	m, err := readManifest(os.DirFS(dir), ".")
	if isErr {
		s.Error(err)
		return
	}
	s.NoError(err)
	s.Equal(currVer, m.CurrVersion)
	s.Equal(minVer, m.MinCompatibleVersion)
	s.Equal(desc, m.Description)
	s.True(len(m.md5) > 0)
	s.Equal(files, m.SchemaUpdateCqlFiles)
}
