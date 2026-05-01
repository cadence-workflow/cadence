// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	scenarioFilter = flag.String("scenarios", ".*", "Regex to filter the tests to execute by name")
	mode           = flag.String("mode", "RunAndCompare", "Mode to run the tool in. Options: RunAndCompare, Compare. "+
		"RunAndCompare runs the simulation and compares the results. "+
		"Compare only compares the results of previously run results. Compare requires --ts flag to be set.")
	timestamp = flag.String("ts", "", "Timestamp of the simulation run to compare in following format '2006-01-02-15-04-05'. Required when mode is Compare")
)

const (
	outputFolder = "matching-simulator-output"
)

var (
	simNameRegex = regexp.MustCompile(`matching_simulation_(?P<name>.*).yaml`)

	// oneLineStatRegex extracts key value pairs that represent a single line stat from the summary file
	// e.g. Max Task latency (ms): 8273
	//
	// See https://regex101.com/r/ZqeiWC/1 for example matches
	oneLineStatRegex = regexp.MustCompile(`^(?P<key>[a-zA-Z]+[a-zA-Z0-9\s\(\)]+):[\s]+(?P<val>[+-]?[0-9]*[.]?[0-9]+[s]?)$`)

	// multiLineStatRegex matches with the start of a multi line stat in the summary file.
	// Subsequent lines that start with a space are part of the same stat and parsed in the code.
	// e.g.
	// Per tasklist sync matches:
	//   179 "/__cadence_sys/my-tasklist/1"
	//   375 "/__cadence_sys/my-tasklist/2"
	//   470 "/__cadence_sys/my-tasklist/3"
	//   3222 "my-tasklist"
	//
	// See https://regex101.com/r/Un3yLo/1 for example matches
	multiLineStatRegex = regexp.MustCompile(`(?m)^(?P<multilinestart>[a-zA-Z]+[a-zA-Z0-9\s\(\)]+):[\s]?$`)
)

func main() {
	validateAndParseFlags()

	root := mustGetRootDir()

	scenarios := mustGetSimulationScenarios(root)

	ts := time.Now().UTC().Format("2006-01-02-15-04-05")
	if *timestamp != "" {
		ts = *timestamp
	}

	if *mode == "RunAndCompare" {
		for _, scenario := range scenarios {
			mustRunScenario(root, scenario, ts)
		}
	}

	mustGenerateReports(root, scenarios, ts)
}

func mustGenerateReports(root string, scenarios []string, ts string) {
	// outer key is scenario name, inner key is stat name, value is stat value
	csvData := make(map[string]map[string]string)

	var missingScenarios []string
	var missingScenariosReasons []string
	for _, scenario := range scenarios {
		if reason, ok := scenarioHasRun(root, scenario, ts); !ok {
			missingScenarios = append(missingScenarios, scenario)
			missingScenariosReasons = append(missingScenariosReasons, reason)
			continue
		}

		summaryFile := scenarioSummaryFile(root, scenario, ts)
		csvData[scenario] = mustParseSummaryFile(summaryFile, scenario)
	}

	if len(missingScenarios) == len(scenarios) {
		log.Fatalf("No simulation results found for any of the scenarios for timestamp: %s, reasons:\n%s", ts, strings.Join(missingScenariosReasons, "\n"))
	}

	allStatKeys := mustGetAllStatKeys(csvData)
	headers := append([]string{"scenario"}, allStatKeys...)
	var data [][]string
	outputFile := csvFilePath(root)
	writer := mustNewCSVWriter(csvFilePath(root))
	for scenario, stats := range csvData {
		row := []string{scenario}
		for _, key := range allStatKeys {
			row = append(row, stats[key])
		}

		data = append(data, row)
	}

	writer.Write(headers)
	for _, row := range data {
		writer.Write(row)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Fatalf("Error writing to output file, err: %v", err)
	}

	fmt.Printf("Comparison CSV generated at: %s\n", outputFile)
}

func csvFilePath(root string) string {
	return path.Join(root, outputFolder, "comparison.csv")
}

func mustGetAllStatKeys(csvData map[string]map[string]string) []string {
	allStatKeys := make(map[string]bool)
	for _, stats := range csvData {
		for k := range stats {
			allStatKeys[k] = true
		}
	}

	var allStatKeysSlice []string
	for k := range allStatKeys {
		allStatKeysSlice = append(allStatKeysSlice, k)
	}

	sort.Strings(allStatKeysSlice)

	return allStatKeysSlice
}

func mustParseSummaryFile(path, scenario string) map[string]string {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Could not read file %s, err: %v", path, err)
	}
	strContent := string(content)
	stats := make(map[string]string)

	// extract one line stats
	for _, line := range strings.Split(strContent, "\n") {
		matches := oneLineStatRegex.FindStringSubmatch(line)
		if len(matches) == 0 {
			continue
		}
		stats[matches[oneLineStatRegex.SubexpIndex("key")]] = matches[oneLineStatRegex.SubexpIndex("val")]
	}

	fmt.Printf("Scenario %q has %d oneline stats\n", scenario, len(stats))

	// extract multi line stats
	indices := multiLineStatRegex.FindAllStringIndex(strContent, -1)
	fmt.Printf("Scenario %q has %d multiline stats\n", scenario, len(indices))
	for _, idx := range indices {
		start := idx[0]
		end := idx[1]
		key := strContent[start:end]

		// value is all lines that start with a space after the key
		var b bytes.Buffer
		rest := strContent[end:]
		for i := strings.Index(rest, "\n"); i < len(rest); i++ {
			if rest[i] == '\n' && i+1 < len(rest) && rest[i+1] != ' ' {
				break
			}

			b.WriteByte(rest[i])
		}

		stats[key] = b.String()
	}

	return stats
}

func mustNewCSVWriter(outputFile string) *csv.Writer {
	file, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Could not create output file, err: %v", err)
	}

	return csv.NewWriter(file)
}

func scenarioSummaryFile(root, scenario, ts string) string {
	// e.g. matching-simulator-output/test-default-2024-09-12-18-16-44-summary.txt
	return path.Join(root, fmt.Sprintf("matching-simulator-output/test-%s-%s-summary.txt", scenario, ts))
}

func scenarioRawEventsFile(root, scenario, ts string) string {
	// e.g. matching-simulator-output/test-default-2024-09-12-18-16-44-events.json
	return path.Join(root, fmt.Sprintf("matching-simulator-output/test-%s-%s-events.json", scenario, ts))
}

func mustRunScenario(root, scenario, ts string) {
	if _, ok := scenarioHasRun(root, scenario, ts); ok {
		fmt.Printf("Scenario %s already ran for timestamp %s, skipping\n", scenario, ts)
		return
	}

	fmt.Printf("Running scenario: %s\n", scenario)
	start := time.Now()
	cmd := exec.Command("bash", path.Join(root, "simulation/matching/run.sh"), "--scenario", scenario, "--timestamp", ts)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		log.Fatalf("Could not run scenario %s, err: %v,\n-----stdout:-----\n%s\n----stderr:----\n%s\nMore details can be found in test.log file.\n", scenario, err, stdout.String(), stderr.String())
	}

	fmt.Printf("Finished running scenario: %s in %v seconds\n", scenario, time.Since(start).Seconds())
}

func mustGetSimulationScenarios(root string) []string {
	path := path.Join(root, "simulation/matching/testdata")
	entries, err := os.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}

	scenarioFilterRegex, err := regexp.Compile(*scenarioFilter)
	if err != nil {
		log.Fatalf("Error parsing scenarios flag: %v", err)
	}

	var scenarios []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := simNameRegex.FindStringSubmatch(entry.Name())
		if len(matches) == 0 {
			continue
		}
		scenarioName := matches[simNameRegex.SubexpIndex("name")]
		if scenarioFilterRegex.MatchString(scenarioName) {
			scenarios = append(scenarios, scenarioName)
		} else {
			log.Printf("Skipping %s due to scenario filter", scenarioName)
		}

	}

	fmt.Println("Simulation scenarios found: \n  ", strings.Join(scenarios, "\n   "))
	return scenarios
}

func validateAndParseFlags() {
	flag.Parse()

	if *mode != "RunAndCompare" && *mode != "Compare" {
		fmt.Println("--mode must be RunAndCompare or Compare")
		os.Exit(1)
	}

	if *mode == "Compare" && *timestamp == "" {
		fmt.Println("--ts is required when mode is Compare")
		os.Exit(1)
	}
}

func scenarioHasRun(root, scenario, ts string) (string, bool) {
	// check if summary file exists and complete
	summaryFilePath := scenarioSummaryFile(root, scenario, ts)
	_, err := os.Stat(summaryFilePath)
	if os.IsNotExist(err) {
		return fmt.Sprintf("summary file %s doesn't exist", summaryFilePath), false
	}

	content, err := os.ReadFile(summaryFilePath)
	if err != nil {
		log.Fatalf("Could not read summary file %s, err: %v", summaryFilePath, err)
	}

	if !strings.Contains(string(content), "End of summary") {
		return fmt.Sprintf("summary file %s doesn't contain 'End of summary'", summaryFilePath), false
	}

	// check if raw events file exists
	eventFilePath := scenarioRawEventsFile(root, scenario, ts)
	_, err = os.Stat(eventFilePath)
	if os.IsNotExist(err) {
		return fmt.Sprintf("event file %s doesn't exist", eventFilePath), false
	}

	return "", true
}

func mustGetRootDir() string {
	root, err := os.Getwd()
	if err != nil {
		log.Fatalf("Could not get executable path, err: %v", err)
	}

	return root
}
