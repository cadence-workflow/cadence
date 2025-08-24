package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/uber/cadence/internal/tools/metricnames"
)

func main() {
	singlechecker.Main(metricnames.Analyzer)
}
