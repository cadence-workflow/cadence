package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/uber/cadence/internal/tools/metricnames"
)

func main() {
	var analyze bool
	flag.BoolVar(&analyze, "analyze", false, "if true, run the analyzer normally.  if false, run the analyzer internally and interpret the results as a linter")
	flag.Parse()

	if analyze {
		singlechecker.Main(metricnames.Analyzer)
	} else {
		os.Exit(checkAnalyzerOutput())
	}
}

func checkAnalyzerOutput() (exitCode int) {
	cmd := exec.Command(os.Args[0], append([]string{"-analyze"}, os.Args[1:]...)...)
	out, _ := cmd.CombinedOutput()

	lines := strings.Split(string(out), "\n")
	// map of metric name to set of lines using it (to deduplicate)
	names := make(map[string]map[string]struct{}, len(lines))
	var failures []string
	for _, line := range lines {
		if line == "" {
			continue // empty lines are fine
		}
		words := strings.Fields(line)
		if len(words) == 3 && words[1] == "success:" {
			prev, ok := names[words[2]]
			if !ok {
				prev = make(map[string]struct{})
			}
			prev[line] = struct{}{}
			names[words[2]] = prev
		} else {
			failures = append(failures, words[0]+" "+words[2]) // skip the "success:" term
		}
	}

	if len(failures) > 0 {
		_, _ = fmt.Fprint(os.Stderr, "Metric name linter encountered errors:")
		_, _ = fmt.Fprint(os.Stderr, strings.Join(failures, "\n")+"\n")
		return 1
	}

	exitCode = 0
	for name, usages := range names {
		if len(usages) > 1 {
			_, _ = fmt.Fprintf(os.Stderr, "Metric name %q used in multiple places:\n", name)
			// frustratingly: this can't use `maps.Keys` because it
			// returns an iterator, not a slice, so it can't be sorted.
			// bleh.
			sorted := make([]string, 0, len(usages))
			for usage := range usages {
				sorted = append(sorted, usage)
			}
			slices.Sort(sorted)
			for usage := range usages {
				_, _ = fmt.Fprintf(os.Stderr, "\t%s\n", usage)
			}
			exitCode = 1
		}
	}

	return exitCode
}
