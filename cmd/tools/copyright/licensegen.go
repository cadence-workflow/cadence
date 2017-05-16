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

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// task that adds license header to source
// files, if they don't already exist
type addLicenseHeaderTask struct {
	license string // license header string to add
	rootDir string // root directory of the project source
}

// licenseFileName is the name of the license file
const licenseFileName = "LICENSE"

// unique prefix that identifies a license header
const licenseHeaderPrefix = "// Copyright (c)"

var (
	// directories to be excluded
	dirBlacklist = []string{"vendor/", "go-build/"}
	// default perms for the newly created files
	defaultFilePerms = os.FileMode(0644)
)

// command line utility that adds license header
// to the source files. Usage as follows:
//
//  ./cmd/tools/copyright/licensegen.go
func main() {
	task := newAddLicenseHeaderTask(".")
	if err := task.run(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func newAddLicenseHeaderTask(rootDir string) *addLicenseHeaderTask {
	return &addLicenseHeaderTask{
		rootDir: rootDir,
	}
}

func (task *addLicenseHeaderTask) run() error {
	data, err := ioutil.ReadFile(task.rootDir + "/" + licenseFileName)
	if err != nil {
		return fmt.Errorf("error reading license file, errr=%v", err.Error())
	}

	task.license = string(data)

	err = filepath.Walk(task.rootDir, task.handleFile)
	if err != nil {
		return fmt.Errorf("error processing files, err=%v", err.Error())
	}
	return nil
}

func (task *addLicenseHeaderTask) handleFile(path string, fileInfo os.FileInfo, err error) error {

	if err != nil {
		return err
	}

	if fileInfo.IsDir() {
		return nil
	}

	if !mustProcessPath(path) {
		return nil
	}

	if !strings.HasSuffix(fileInfo.Name(), ".go") {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}

	buf := make([]byte, len(licenseHeaderPrefix))
	_, err = io.ReadFull(f, buf)
	f.Close()

	if err != nil && !isEOF(err) {
		return err
	}

	if string(buf) == licenseHeaderPrefix {
		return nil
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, []byte(task.license+string(data)), defaultFilePerms)
}

func mustProcessPath(path string) bool {
	for _, d := range dirBlacklist {
		if strings.HasPrefix(path, d) {
			return false
		}
	}
	return true
}

// returns true if the error type is an EOF
func isEOF(err error) bool {
	return err == io.EOF || err == io.ErrUnexpectedEOF
}
