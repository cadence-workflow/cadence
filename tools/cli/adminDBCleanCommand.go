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

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/gocql/gocql"
	"github.com/urfave/cli"

	"github.com/uber/cadence/common/log/loggerimpl"
	"github.com/uber/cadence/common/persistence"
	cassp "github.com/uber/cadence/common/persistence/cassandra"
	"github.com/uber/cadence/common/quotas"
)

type (
	// ShardCleanReport represents the result of cleaning a single shard
	ShardCleanReport struct {
		ShardID         int
		TotalDBRequests int64
		Handled         *ShardCleanReportHandled
		Failure         *ShardCleanReportFailure
	}

	// ShardCleanReportHandled is the part of ShardCleanReport of executions which were read from corruption file
	// and were attempted to be deleted
	ShardCleanReportHandled struct {
		TotalExecutionsCount          int64
		SuccessfullyCleanedCount      int64
		FailedCleanedCount            int64
		FailedToConfirmCorruptedCount int64
	}

	// ShardCleanReportFailure is the part of ShardCleanReport that indicates a failure to clean some or all
	// of the executions found in corruption file
	ShardCleanReportFailure struct {
		Note    string
		Details string
	}

	// CleanProgressReport represents the aggregate progress of the clean job.
	// It is periodically printed to stdout
	// TODO: move these reports into there own file like we did for scan
	CleanProgressReport struct {
		NumberOfShardsFinished        int
		TotalExecutionsCount          int64
		SuccessfullyCleanedCount      int64
		FailedCleanedCount            int64
		FailedToConfirmCorruptedCount int64
		TotalDBRequests               int64
		DatabaseRPS                   float64
		NumberOfShardCleanFailures    int64
		ShardsPerHour                 float64
		ExecutionsPerHour             float64
	}

	// CleanOutputDirectories are the directory paths for output of clean
	CleanOutputDirectories struct {
		ShardCleanReportDirectoryPath    string
		SuccessfullyCleanedDirectoryPath string
		FailedCleanedDirectoryPath       string
	}

	// ShardCleanOutputFiles are the files produced for a clean of a single shard
	ShardCleanOutputFiles struct {
		ShardCleanReportFile    *os.File
		SuccessfullyCleanedFile *os.File
		FailedCleanedFile       *os.File
	}
)

// AdminDBClean is the command to clean up executions
func AdminDBClean(c *cli.Context) {
	lowerShardBound := c.Int(FlagLowerShardBound)
	upperShardBound := c.Int(FlagUpperShardBound)
	numShards := upperShardBound - lowerShardBound
	startingRPS := c.Int(FlagStartingRPS)
	targetRPS := c.Int(FlagRPS)
	scaleUpSeconds := c.Int(FlagRPSScaleUpSeconds)
	scanWorkerCount := c.Int(FlagConcurrency)
	scanReportRate := c.Int(FlagReportRate)
	if numShards < scanWorkerCount {
		scanWorkerCount = numShards
	}
	inputDirectory := getRequiredOption(c, FlagInputDirectory)
	skipHistoryChecks := c.Bool(FlagSkipHistoryChecks)

	payloadSerializer := persistence.NewPayloadSerializer()
	rateLimiter := getRateLimiter(startingRPS, targetRPS, scaleUpSeconds)
	session := connectToCassandra(c)
	defer session.Close()
	historyStore := cassp.NewHistoryV2PersistenceFromSession(session, loggerimpl.NewNopLogger())
	cleanOutputDirectories := createCleanOutputDirectories()

	reports := make(chan *ShardCleanReport)
	for i := 0; i < scanWorkerCount; i++ {
		go func(workerIdx int) {
			for shardID := lowerShardBound; shardID < upperShardBound; shardID++ {
				if shardID%scanWorkerCount == workerIdx {
					reports <- cleanShard(
						rateLimiter,
						session,
						cleanOutputDirectories,
						inputDirectory,
						shardID,
						skipHistoryChecks,
						payloadSerializer,
						historyStore)
				}
			}
		}(i)
	}

	startTime := time.Now()
	progressReport := &CleanProgressReport{}
	for i := 0; i < numShards; i++ {
		report := <-reports
		includeShardCleanInProgressReport(report, progressReport, startTime)
		if i%scanReportRate == 0 || i == numShards-1 {
			reportBytes, err := json.MarshalIndent(*progressReport, "", "\t")
			if err != nil {
				ErrorAndExit("failed to print progress", err)
			}
			fmt.Println(string(reportBytes))
		}
	}
}

func cleanShard(
	limiter *quotas.DynamicRateLimiter,
	session *gocql.Session,
	outputDirectories *CleanOutputDirectories,
	inputDirectory string,
	shardID int,
	skipHistoryChecks bool,
	payloadSerializer persistence.PayloadSerializer,
	historyStore persistence.HistoryStore,
) *ShardCleanReport {
	outputFiles, closeFn := createShardCleanOutputFiles(shardID, outputDirectories)
	report := &ShardCleanReport{
		ShardID: shardID,
	}
	failedCleanWriter := NewBufferedWriter(outputFiles.FailedCleanedFile)
	successfullyCleanWriter := NewBufferedWriter(outputFiles.SuccessfullyCleanedFile)
	defer func() {
		failedCleanWriter.Flush()
		successfullyCleanWriter.Flush()
		recordShardCleanReport(outputFiles.ShardCleanReportFile, report)
		deleteEmptyFiles(outputFiles.ShardCleanReportFile, outputFiles.SuccessfullyCleanedFile, outputFiles.FailedCleanedFile)
		closeFn()
	}()
	shardCorruptedFile, err := getShardCorruptedFile(inputDirectory, shardID)
	if err != nil {
		if !os.IsNotExist(err) {
			report.Failure = &ShardCleanReportFailure{
				Note:    "failed to get corruption file",
				Details: err.Error(),
			}
		}
		return report
	}
	defer shardCorruptedFile.Close()
	execStore, err := cassp.NewWorkflowExecutionPersistence(shardID, session, loggerimpl.NewNopLogger())
	if err != nil {
		report.Failure = &ShardCleanReportFailure{
			Note:    "failed to create execution store",
			Details: err.Error(),
		}
		return report
	}

	checks := getChecks(skipHistoryChecks, limiter, execStore, payloadSerializer, historyStore)

	scanner := bufio.NewScanner(shardCorruptedFile)
	for scanner.Scan() {
		if report.Handled == nil {
			report.Handled = &ShardCleanReportHandled{}
		}
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		report.Handled.TotalExecutionsCount++
		var etr ExecutionToRecord
		err := json.Unmarshal([]byte(line), &etr)
		if err != nil {
			report.Handled.FailedCleanedCount++
			continue
		}

		// run checks again to confirm that the execution is still corrupted
		cr := &CheckRequest{
			ShardID:    etr.ShardID,
			DomainID:   etr.DomainID,
			WorkflowID: etr.WorkflowID,
			RunID:      etr.RunID,
			TreeID:     etr.TreeID,
			BranchID:   etr.BranchID,
			State:      etr.State,
		}
		confirmedCorrupted := false
		for _, c := range checks {
			if !c.ValidRequest(cr) {
				continue
			}
			result := c.Check(cr)
			cr.PrerequisiteCheckPayload = result.Payload
			if result.CheckResultStatus == CheckResultCorrupted {
				confirmedCorrupted = true
				break
			}
		}
		if !confirmedCorrupted {
			report.Handled.FailedToConfirmCorruptedCount++
			continue
		}

		deleteConcreteReq := &persistence.DeleteWorkflowExecutionRequest{
			DomainID:   etr.DomainID,
			WorkflowID: etr.WorkflowID,
			RunID:      etr.RunID,
		}
		err = retryDeleteWorkflowExecution(limiter, &report.TotalDBRequests, execStore, deleteConcreteReq)
		if err != nil {
			report.Handled.FailedCleanedCount++
			failedCleanWriter.Add(&etr)
			continue
		}
		report.Handled.SuccessfullyCleanedCount++
		successfullyCleanWriter.Add(&etr)
		deleteCurrentReq := &persistence.DeleteCurrentWorkflowExecutionRequest{
			DomainID:   etr.DomainID,
			WorkflowID: etr.WorkflowID,
			RunID:      etr.RunID,
		}
		// deleting current execution is best effort, the success or failure of the cleanup
		// is determined above based on if the concrete execution could be deleted
		retryDeleteCurrentWorkflowExecution(limiter, &report.TotalDBRequests, execStore, deleteCurrentReq)

		// TODO: also need to clean up histories of corrupted workflows
	}
	return report
}

func getShardCorruptedFile(inputDir string, shardID int) (*os.File, error) {
	filepath := fmt.Sprintf("%v/%v", inputDir, constructFileNameFromShard(shardID))
	return os.Open(filepath)
}

func includeShardCleanInProgressReport(report *ShardCleanReport, progressReport *CleanProgressReport, startTime time.Time) {
	progressReport.NumberOfShardsFinished++
	progressReport.TotalDBRequests += report.TotalDBRequests
	if report.Failure != nil {
		progressReport.NumberOfShardCleanFailures++
	}

	if report.Handled != nil {
		progressReport.TotalExecutionsCount += report.Handled.TotalExecutionsCount
		progressReport.FailedCleanedCount += report.Handled.FailedCleanedCount
		progressReport.FailedToConfirmCorruptedCount += report.Handled.FailedToConfirmCorruptedCount
		progressReport.SuccessfullyCleanedCount += report.Handled.SuccessfullyCleanedCount
	}

	pastTime := time.Now().Sub(startTime)
	hoursPast := float64(pastTime) / float64(time.Hour)
	progressReport.ShardsPerHour = math.Round(float64(progressReport.NumberOfShardsFinished) / hoursPast)
	progressReport.ExecutionsPerHour = math.Round(float64(progressReport.TotalExecutionsCount) / hoursPast)
	secondsPast := float64(pastTime) / float64(time.Second)
	progressReport.DatabaseRPS = math.Round(float64(progressReport.TotalDBRequests) / secondsPast)
}

func createShardCleanOutputFiles(shardID int, cod *CleanOutputDirectories) (*ShardCleanOutputFiles, func()) {
	shardCleanReportFile, err := os.Create(fmt.Sprintf("%v/%v", cod.ShardCleanReportDirectoryPath, constructFileNameFromShard(shardID)))
	if err != nil {
		ErrorAndExit("failed to create ShardCleanReportFile", err)
	}
	successfullyCleanedFile, err := os.Create(fmt.Sprintf("%v/%v", cod.SuccessfullyCleanedDirectoryPath, constructFileNameFromShard(shardID)))
	if err != nil {
		ErrorAndExit("failed to create SuccessfullyCleanedFile", err)
	}
	failedCleanedFile, err := os.Create(fmt.Sprintf("%v/%v", cod.FailedCleanedDirectoryPath, constructFileNameFromShard(shardID)))
	if err != nil {
		ErrorAndExit("failed to create FailedCleanedFile", err)
	}

	deferFn := func() {
		shardCleanReportFile.Close()
		successfullyCleanedFile.Close()
		failedCleanedFile.Close()
	}
	return &ShardCleanOutputFiles{
		ShardCleanReportFile:    shardCleanReportFile,
		SuccessfullyCleanedFile: successfullyCleanedFile,
		FailedCleanedFile:       failedCleanedFile,
	}, deferFn
}

func createCleanOutputDirectories() *CleanOutputDirectories {
	now := time.Now().Unix()
	cod := &CleanOutputDirectories{
		ShardCleanReportDirectoryPath:    fmt.Sprintf("./clean_%v/shard_clean_report", now),
		SuccessfullyCleanedDirectoryPath: fmt.Sprintf("./clean_%v/successfully_cleaned", now),
		FailedCleanedDirectoryPath:       fmt.Sprintf("./clean_%v/failed_cleaned", now),
	}
	if err := os.MkdirAll(cod.ShardCleanReportDirectoryPath, 0777); err != nil {
		ErrorAndExit("failed to create ShardCleanReportDirectoryPath", err)
	}
	if err := os.MkdirAll(cod.SuccessfullyCleanedDirectoryPath, 0777); err != nil {
		ErrorAndExit("failed to create SuccessfullyCleanedDirectoryPath", err)
	}
	if err := os.MkdirAll(cod.FailedCleanedDirectoryPath, 0777); err != nil {
		ErrorAndExit("failed to create FailedCleanedDirectoryPath", err)
	}
	fmt.Println("clean results located under: ", fmt.Sprintf("./clean_%v", now))
	return cod
}

func recordShardCleanReport(file *os.File, sdr *ShardCleanReport) {
	data, err := json.Marshal(sdr)
	if err != nil {
		ErrorAndExit("failed to marshal ShardCleanReport", err)
	}
	writeToFile(file, string(data))
}
