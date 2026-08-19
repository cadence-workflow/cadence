package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/uber/cadence/tools/dependencyage"
)

const (
	defaultThresholdDays = 14
	checkTimeout         = 10 * time.Minute
)

func main() {
	baseRef := flag.String("base-ref", "", "git ref to diff against (e.g. origin/master)")
	flag.Parse()
	if strings.TrimSpace(*baseRef) == "" {
		flag.Usage()
		os.Exit(2)
	}

	rawThreshold := strings.TrimSpace(os.Getenv("MIN_DEPENDENCY_AGE_DAYS"))
	thresholdDays := defaultThresholdDays
	if rawThreshold != "" {
		var err error
		thresholdDays, err = strconv.Atoi(rawThreshold)
		if err != nil {
			_, _ = fmt.Fprintf(
				os.Stderr,
				"ERROR MIN_DEPENDENCY_AGE_DAYS must be an integer, got %q\n",
				rawThreshold,
			)
			os.Exit(2)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	fetcher := &dependencyage.GoListFetcher{}
	os.Exit(dependencyage.Run(ctx, dependencyage.Config{
		BaseRef:       *baseRef,
		ThresholdDays: thresholdDays,
		Fetch:         fetcher.PublishTime,
		Source:        &dependencyage.GitSource{},
		Now:           time.Now().UTC(),
		Out:           os.Stdout,
		Err:           os.Stderr,
	}))
}
