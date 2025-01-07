// The MIT License (MIT)
//
// Copyright (c) 2020 Uber Technologies, Inc.
//
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

//go:generate mockgen -package $GOPACKAGE -destination admin_timers_mock.go -self_package github.com/uber/cadence/tools/cli github.com/uber/cadence/tools/cli LoadCloser,Printer

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/tools/common/commoncli"
)

// LoadCloser loads timer task information
type LoadCloser interface {
	Load() ([]*persistence.TimerTaskInfo, error)
	Close()
}

// Printer prints timer task information
type Printer interface {
	Print(output io.Writer, timers []*persistence.TimerTaskInfo) error
}

// Reporter wraps LoadCloser, Printer and a filter on time task type and domainID
type Reporter struct {
	domainID   string
	timerTypes []int
	loader     LoadCloser
	printer    Printer
}

type dbLoadCloser struct {
	ctx              *cli.Context
	executionManager persistence.ExecutionManager
}

type fileLoadCloser struct {
	file *os.File
}

type histogramPrinter struct {
	ctx        *cli.Context
	timeFormat string
}

type jsonPrinter struct {
	ctx *cli.Context
}

// NewDBLoadCloser creates a new LoadCloser to load timer task information from database
func NewDBLoadCloser(c *cli.Context) (LoadCloser, error) {
	shardID, err := getRequiredIntOption(c, FlagShardID)
	if err != nil {
		return nil, fmt.Errorf("error in NewDBLoadCloser: failed to get shard ID: %w", err)
	}

	executionManager, err := getDeps(c).initializeExecutionManager(c, shardID)
	if err != nil {
		return nil, fmt.Errorf("error in NewDBLoadCloser: failed to initialize execution store: %w", err)
	}
	return &dbLoadCloser{
		ctx:              c,
		executionManager: executionManager,
	}, nil
}

func NewFileLoadCloser(c *cli.Context) (LoadCloser, error) {
	file, err := os.Open(c.String(FlagInputFile))
	if err != nil {
		return nil, fmt.Errorf("error in NewFileLoadCloser: cannot open file: %w", err)
	}
	return &fileLoadCloser{
		file: file,
	}, nil
}

// NewReporter creates a new Reporter
func NewReporter(domain string, timerTypes []int, loader LoadCloser, printer Printer) *Reporter {
	return &Reporter{
		timerTypes: timerTypes,
		domainID:   domain,
		loader:     loader,
		printer:    printer,
	}
}

// NewHistogramPrinter creates a new Printer to display timer task information in a histogram
func NewHistogramPrinter(c *cli.Context, timeFormat string) Printer {
	return &histogramPrinter{
		ctx:        c,
		timeFormat: timeFormat,
	}
}

// NewJSONPrinter creates a new Printer to display timer task information in a JSON format
func NewJSONPrinter(c *cli.Context) Printer {
	return &jsonPrinter{
		ctx: c,
	}
}

func (r *Reporter) filter(timers []*persistence.TimerTaskInfo) []*persistence.TimerTaskInfo {
	taskTypes := intSliceToSet(r.timerTypes)

	for i, t := range timers {
		if len(r.domainID) > 0 && t.DomainID != r.domainID {
			timers[i] = nil
			continue
		}
		if _, ok := taskTypes[t.TaskType]; !ok {
			timers[i] = nil
			continue

		}
	}

	return timers
}

// Report loads, filters and prints timer tasks
func (r *Reporter) Report(output io.Writer) error {
	loader, err := r.loader.Load()
	if err != nil {
		return err
	}
	return r.printer.Print(output, r.filter(loader))
}

// AdminTimers is used to list scheduled timers.
func AdminTimers(c *cli.Context) error {
	timerTypes := c.IntSlice(FlagTimerType)
	if !c.IsSet(FlagTimerType) || (len(timerTypes) == 1 && timerTypes[0] == -1) {
		timerTypes = []int{
			persistence.TaskTypeDecisionTimeout,
			persistence.TaskTypeActivityTimeout,
			persistence.TaskTypeUserTimer,
			persistence.TaskTypeWorkflowTimeout,
			persistence.TaskTypeDeleteHistoryEvent,
			persistence.TaskTypeActivityRetryTimer,
			persistence.TaskTypeWorkflowBackoffTimer,
		}
	}

	// setup loader
	var loader LoadCloser
	var err error
	if !c.IsSet(FlagInputFile) {
		loader, err = NewDBLoadCloser(c)
		if err != nil {
			return commoncli.Problem("Error in timer: ", err)
		}
	} else {
		loader, err = NewFileLoadCloser(c)
		if err != nil {
			return commoncli.Problem("Error in timer: ", err)
		}
	}
	defer loader.Close()

	// setup printer
	var printer Printer
	if !c.Bool(FlagPrintJSON) {
		var timerFormat string
		if c.IsSet(FlagDateFormat) {
			timerFormat = c.String(FlagDateFormat)
		} else {
			switch c.String(FlagBucketSize) {
			case "day":
				timerFormat = "2006-01-02"
			case "hour":
				timerFormat = "2006-01-02T15"
			case "minute":
				timerFormat = "2006-01-02T15:04"
			case "second":
				timerFormat = "2006-01-02T15:04:05"
			default:
				return commoncli.Problem("unknown bucket size: "+c.String(FlagBucketSize), nil)
			}
		}
		printer = NewHistogramPrinter(c, timerFormat)
	} else {
		printer = NewJSONPrinter(c)
	}

	reporter := NewReporter(c.String(FlagDomainID), timerTypes, loader, printer)
	output := getDeps(c).Output()
	if err := reporter.Report(output); err != nil {
		return commoncli.Problem("Reporter failed", err)
	}
	return nil
}

func (jp *jsonPrinter) Print(output io.Writer, timers []*persistence.TimerTaskInfo) error {
	for _, t := range timers {
		if t == nil {
			continue
		}
		data, err := json.Marshal(t)
		if err != nil {
			if !jp.ctx.Bool(FlagSkipErrorMode) {
				return commoncli.Problem("cannot marshal timer to json", err)
			}
			output.Write([]byte(fmt.Sprintf("%s\n", err.Error())))
		} else {
			output.Write([]byte(fmt.Sprintf("%s\n", data)))
		}
	}
	return nil
}

func (cl *dbLoadCloser) Load() ([]*persistence.TimerTaskInfo, error) {
	batchSize := cl.ctx.Int(FlagBatchSize)
	startDate := cl.ctx.String(FlagStartDate)
	endDate := cl.ctx.String(FlagEndDate)

	st, err := parseSingleTs(startDate)
	if err != nil {
		return nil, fmt.Errorf("wrong date format for "+FlagEndDate+" Error: %v", err)
	}
	et, err := parseSingleTs(endDate)
	if err != nil {
		return nil, fmt.Errorf("wrong date format for "+FlagEndDate+" Error: %v", err)
	}

	var timers []*persistence.TimerTaskInfo

	isRetryable := func(err error) bool {
		return persistence.IsTransientError(err) || common.IsContextTimeoutError(err)
	}

	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(common.CreatePersistenceRetryPolicy()),
		backoff.WithRetryableError(isRetryable),
	)
	var token []byte
	isFirstIteration := true
	for isFirstIteration || len(token) != 0 {
		isFirstIteration = false
		req := persistence.GetTimerIndexTasksRequest{
			MinTimestamp:  st,
			MaxTimestamp:  et,
			BatchSize:     batchSize,
			NextPageToken: token,
		}

		resp := &persistence.GetTimerIndexTasksResponse{}

		op := func() error {
			ctx, cancel, err := newContext(cl.ctx)
			defer cancel()
			if err != nil {
				return commoncli.Problem("Error in creating context:", err)
			}
			resp, err = cl.executionManager.GetTimerIndexTasks(ctx, &req)
			return err
		}

		err = throttleRetry.Do(cl.ctx.Context, op)

		if err != nil {
			return nil, fmt.Errorf("cannot get timer tasks for shard: %v", err)
		}

		token = resp.NextPageToken
		timers = append(timers, resp.Timers...)
	}
	return timers, nil
}

func (cl *dbLoadCloser) Close() {
	if cl.executionManager != nil {
		cl.executionManager.Close()
	}
}

func (fl *fileLoadCloser) Load() ([]*persistence.TimerTaskInfo, error) {
	var data []*persistence.TimerTaskInfo
	dec := json.NewDecoder(fl.file)

	for {
		var timer persistence.TimerTaskInfo
		if err := dec.Decode(&timer); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return nil, fmt.Errorf("error decoding timer: %w", err)
		}
		data = append(data, &timer)
	}
	return data, nil
}

func (fl *fileLoadCloser) Close() {
	if fl.file != nil {
		fl.file.Close()
	}
}

func (hp *histogramPrinter) Print(output io.Writer, timers []*persistence.TimerTaskInfo) error {
	h := NewHistogram()
	for _, t := range timers {
		if t == nil {
			continue
		}
		h.Add(t.VisibilityTimestamp.Format(hp.timeFormat))
	}

	return h.Print(output, hp.ctx.Int(FlagShardMultiplier))
}
