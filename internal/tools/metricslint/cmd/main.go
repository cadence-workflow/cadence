package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/uber/cadence/internal/tools/metricslint"
)

func main() {
	var analyze bool
	flag.BoolVar(&analyze, "analyze", false, "if true, run the analyzer normally (e.g. for debugging).  if false, run the analyzer internally and interpret the results as a linter")
	skip := map[string]int{}
	flag.Func("skip", "metric name to ignore, comma, followed by the number of times it is duplicated.  repeat to skip more things.", func(s string) error {
		parts := strings.SplitN(s, ",", 2)
		if len(parts) != 2 {
			return fmt.Errorf("skip argument %q is not in the form `name,count`", s)
		}
		count, err := strconv.Atoi(parts[1])
		if err != nil || count < 2 {
			return fmt.Errorf("skip argument %q has invalid count %q: %v", s, parts[1], err)
		}
		skip[parts[0]] = count
		return nil
	})
	flag.Parse()

	if analyze {
		singlechecker.Main(metricslint.Analyzer)
	} else {
		os.Exit(checkAnalyzerOutput(skip))
	}
}

func checkAnalyzerOutput(skip map[string]int) (exitCode int) {
	cmd := exec.Command(os.Args[0], append([]string{"-analyze"}, os.Args[1:]...)...)
	out, _ := cmd.CombinedOutput()

	defer func() {
		// in case of crashes, print the output we got so far as it likely narrows down the cause.
		if r := recover(); r != nil {
			_, _ = fmt.Fprint(os.Stderr, "\n\n"+string(out))
			panic(r)
		}
	}()

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
			failures = append(failures, line)
		}
	}

	if len(failures) > 0 {
		_, _ = fmt.Fprintln(os.Stderr, strings.Join(failures, "\n")+"\n")
		return 1
	}

	exitCode = 0
	for name, usages := range names {
		var complain bool
		expected, ok := skip[name]
		if ok {
			complain = len(usages) != expected // expectation mismatch
		} else {
			complain = len(usages) > 1 // must not have duplicates
		}
		if complain {
			if expected > 0 {
				_, _ = fmt.Fprintf(os.Stderr, "Metric name %q used in an unexpected number of places:\n", name)
			} else {
				_, _ = fmt.Fprintf(os.Stderr, "Metric name %q used in multiple places:\n", name)
			}
			// frustratingly: this can't use `maps.Keys` because it
			// returns an iterator, not a slice, so it can't be sorted or `...`-spread.
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
