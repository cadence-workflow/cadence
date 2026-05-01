// Copyright (c) 2026 Uber Technologies, Inc.
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

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cli "github.com/urfave/cli/v2"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/types"
	commoncli "github.com/uber/cadence/tools/common/commoncli"
)

type scheduleCLIImpl struct {
	frontendClient frontend.Client
}

func withScheduleClient(c *cli.Context, cb func(sc *scheduleCLIImpl) error) error {
	if c.String(FlagTransport) != grpcTransport {
		if err := c.Set(FlagTransport, grpcTransport); err != nil {
			return commoncli.Problem("Schedule commands require gRPC transport but failed to set it", err)
		}
	}
	client, err := initializeFrontendClient(c)
	if err != nil {
		return err
	}
	return cb(&scheduleCLIImpl{frontendClient: client})
}

func (sc *scheduleCLIImpl) CreateSchedule(c *cli.Context) error {
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return err
	}
	scheduleID := c.String(FlagScheduleID)
	cronExpr := c.String(FlagCronExpression)
	workflowType := c.String(FlagWorkflowType)
	taskList := c.String(FlagTaskList)
	executionTimeout := int32(c.Int(FlagExecutionTimeout))
	decisionTimeout := int32(c.Int(FlagDecisionTimeout))

	action := &types.StartWorkflowAction{
		WorkflowType:                        &types.WorkflowType{Name: workflowType},
		ExecutionStartToCloseTimeoutSeconds: &executionTimeout,
		TaskStartToCloseTimeoutSeconds:      &decisionTimeout,
	}
	if taskList != "" {
		action.TaskList = &types.TaskList{Name: taskList}
	}
	if inputStr := c.String(FlagInput); inputStr != "" {
		if !json.Valid([]byte(inputStr)) {
			return commoncli.Problem("Input is not valid JSON", nil)
		}
		action.Input = []byte(inputStr)
	}

	request := &types.CreateScheduleRequest{
		Domain:     domain,
		ScheduleID: scheduleID,
		Spec:       &types.ScheduleSpec{CronExpression: cronExpr},
		Action:     &types.ScheduleAction{StartWorkflow: action},
	}

	policies, err := buildPoliciesFromFlags(c)
	if err != nil {
		return err
	}
	if policies != nil && policies.ConcurrencyLimit > 0 &&
		policies.OverlapPolicy != types.ScheduleOverlapPolicyConcurrent {
		return commoncli.Problem("--concurrency_limit requires --overlap_policy concurrent", nil)
	}
	if policies != nil {
		request.Policies = policies
	}

	ctx, cancel, err := newContext(c)
	if err != nil {
		return commoncli.Problem("Error creating context", err)
	}
	defer cancel()

	resp, err := sc.frontendClient.CreateSchedule(ctx, request)
	if err != nil {
		return commoncli.Problem("Failed to create schedule", err)
	}

	fmt.Printf("Schedule created successfully. ScheduleID: %s\n", resp.GetScheduleID())
	return nil
}

func (sc *scheduleCLIImpl) DescribeSchedule(c *cli.Context) error {
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return err
	}
	scheduleID := c.String(FlagScheduleID)
	printJSON := c.Bool(FlagPrintJSON)

	ctx, cancel, err := newContext(c)
	if err != nil {
		return commoncli.Problem("Error creating context", err)
	}
	defer cancel()

	resp, err := sc.frontendClient.DescribeSchedule(ctx, &types.DescribeScheduleRequest{
		Domain:     domain,
		ScheduleID: scheduleID,
	})
	if err != nil {
		return commoncli.Problem("Failed to describe schedule", err)
	}

	if printJSON {
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return commoncli.Problem("Failed to marshal response", err)
		}
		fmt.Println(string(data))
		return nil
	}

	printDescribeSchedule(resp)
	return nil
}

func (sc *scheduleCLIImpl) UpdateSchedule(c *cli.Context) error {
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return err
	}
	scheduleID := c.String(FlagScheduleID)

	request := &types.UpdateScheduleRequest{
		Domain:     domain,
		ScheduleID: scheduleID,
	}

	if c.IsSet(FlagCronExpression) {
		request.Spec = &types.ScheduleSpec{CronExpression: c.String(FlagCronExpression)}
	}

	policies, err := buildPoliciesFromFlags(c)
	if err != nil {
		return err
	}
	// Only reject explicit conflicts; standalone --concurrency_limit updates are
	// valid when the schedule's existing server-side policy is already concurrent.
	if policies != nil && policies.ConcurrencyLimit > 0 &&
		c.IsSet(FlagOverlapPolicy) && policies.OverlapPolicy != types.ScheduleOverlapPolicyConcurrent {
		return commoncli.Problem("--concurrency_limit requires --overlap_policy concurrent", nil)
	}
	request.Policies = policies

	if request.Spec == nil && request.Policies == nil {
		return commoncli.Problem("At least one of --cron_expression, --overlap_policy, --catch_up_policy, or --concurrency_limit must be set", nil)
	}

	ctx, cancel, err := newContext(c)
	if err != nil {
		return commoncli.Problem("Error creating context", err)
	}
	defer cancel()

	_, err = sc.frontendClient.UpdateSchedule(ctx, request)
	if err != nil {
		return commoncli.Problem("Failed to update schedule", err)
	}

	fmt.Printf("Schedule %q updated successfully.\n", scheduleID)
	return nil
}

func (sc *scheduleCLIImpl) DeleteSchedule(c *cli.Context) error {
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return err
	}
	scheduleID := c.String(FlagScheduleID)

	ctx, cancel, err := newContext(c)
	if err != nil {
		return commoncli.Problem("Error creating context", err)
	}
	defer cancel()

	_, err = sc.frontendClient.DeleteSchedule(ctx, &types.DeleteScheduleRequest{
		Domain:     domain,
		ScheduleID: scheduleID,
	})
	if err != nil {
		return commoncli.Problem("Failed to delete schedule", err)
	}

	fmt.Printf("Schedule %q deleted successfully.\n", scheduleID)
	return nil
}

func (sc *scheduleCLIImpl) PauseSchedule(c *cli.Context) error {
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return err
	}
	scheduleID := c.String(FlagScheduleID)

	ctx, cancel, err := newContext(c)
	if err != nil {
		return commoncli.Problem("Error creating context", err)
	}
	defer cancel()

	_, err = sc.frontendClient.PauseSchedule(ctx, &types.PauseScheduleRequest{
		Domain:     domain,
		ScheduleID: scheduleID,
		Reason:     c.String(FlagReason),
	})
	if err != nil {
		return commoncli.Problem("Failed to pause schedule", err)
	}

	fmt.Printf("Schedule %q paused successfully.\n", scheduleID)
	return nil
}

func (sc *scheduleCLIImpl) UnpauseSchedule(c *cli.Context) error {
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return err
	}
	scheduleID := c.String(FlagScheduleID)

	request := &types.UnpauseScheduleRequest{
		Domain:     domain,
		ScheduleID: scheduleID,
		Reason:     c.String(FlagReason),
	}

	if c.IsSet(FlagCatchUpPolicy) {
		policy, err := parseCatchUpPolicy(c.String(FlagCatchUpPolicy))
		if err != nil {
			return err
		}
		request.CatchUpPolicy = policy
	}

	ctx, cancel, err := newContext(c)
	if err != nil {
		return commoncli.Problem("Error creating context", err)
	}
	defer cancel()

	_, err = sc.frontendClient.UnpauseSchedule(ctx, request)
	if err != nil {
		return commoncli.Problem("Failed to unpause schedule", err)
	}

	fmt.Printf("Schedule %q unpaused successfully.\n", scheduleID)
	return nil
}

func (sc *scheduleCLIImpl) BackfillSchedule(c *cli.Context) error {
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return err
	}
	scheduleID := c.String(FlagScheduleID)

	startTime, err := time.Parse(time.RFC3339, c.String(FlagStartTime))
	if err != nil {
		return commoncli.Problem("Invalid start_time format, expected RFC3339", err)
	}
	endTime, err := time.Parse(time.RFC3339, c.String(FlagEndTime))
	if err != nil {
		return commoncli.Problem("Invalid end_time format, expected RFC3339", err)
	}
	if !startTime.Before(endTime) {
		return commoncli.Problem("start_time must be before end_time", nil)
	}

	request := &types.BackfillScheduleRequest{
		Domain:     domain,
		ScheduleID: scheduleID,
		StartTime:  startTime,
		EndTime:    endTime,
		BackfillID: c.String(FlagBackfillID),
	}

	if c.IsSet(FlagOverlapPolicy) {
		policy, err := parseOverlapPolicy(c.String(FlagOverlapPolicy))
		if err != nil {
			return err
		}
		request.OverlapPolicy = policy
	}

	ctx, cancel, err := newContext(c)
	if err != nil {
		return commoncli.Problem("Error creating context", err)
	}
	defer cancel()

	_, err = sc.frontendClient.BackfillSchedule(ctx, request)
	if err != nil {
		return commoncli.Problem("Failed to backfill schedule", err)
	}

	fmt.Printf("Backfill requested for schedule %q (%s to %s).\n",
		scheduleID, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))
	return nil
}

func (sc *scheduleCLIImpl) ListSchedules(c *cli.Context) error {
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return err
	}
	pageSize := int32(c.Int(FlagPageSize))

	ctx, cancel, err := newContext(c)
	if err != nil {
		return commoncli.Problem("Error creating context", err)
	}
	defer cancel()

	resp, err := sc.frontendClient.ListSchedules(ctx, &types.ListSchedulesRequest{
		Domain:   domain,
		PageSize: pageSize,
	})
	if err != nil {
		return commoncli.Problem("Failed to list schedules", err)
	}

	if len(resp.GetSchedules()) == 0 {
		fmt.Println("No schedules found.")
		return nil
	}

	for _, entry := range resp.GetSchedules() {
		paused := "active"
		if entry.State != nil && entry.State.Paused {
			paused = "paused"
		}
		wfType := ""
		if entry.WorkflowType != nil {
			wfType = entry.WorkflowType.Name
		}
		fmt.Printf("  %-30s  %-20s  %-20s  %s\n",
			entry.ScheduleID, entry.CronExpression, wfType, paused)
	}
	if len(resp.GetNextPageToken()) > 0 {
		fmt.Println("\n  ... more schedules exist. Use --pagesize to increase the page size.")
	}
	return nil
}

func buildPoliciesFromFlags(c *cli.Context) (*types.SchedulePolicies, error) {
	hasOverlap := c.IsSet(FlagOverlapPolicy)
	hasCatchUp := c.IsSet(FlagCatchUpPolicy)
	hasLimit := c.IsSet(FlagConcurrencyLimit)
	if !hasOverlap && !hasCatchUp && !hasLimit {
		return nil, nil
	}

	policies := &types.SchedulePolicies{}
	if hasOverlap {
		p, err := parseOverlapPolicy(c.String(FlagOverlapPolicy))
		if err != nil {
			return nil, err
		}
		policies.OverlapPolicy = p
	}
	if hasCatchUp {
		p, err := parseCatchUpPolicy(c.String(FlagCatchUpPolicy))
		if err != nil {
			return nil, err
		}
		policies.CatchUpPolicy = p
	}
	if hasLimit {
		limit := int32(c.Int(FlagConcurrencyLimit))
		if limit < 0 {
			return nil, commoncli.Problem("--concurrency_limit must be >= 0", nil)
		}
		policies.ConcurrencyLimit = limit
	}
	return policies, nil
}

func parseOverlapPolicy(s string) (types.ScheduleOverlapPolicy, error) {
	switch strings.ToLower(s) {
	case "skipnew", "skip_new":
		return types.ScheduleOverlapPolicySkipNew, nil
	case "buffer":
		return types.ScheduleOverlapPolicyBuffer, nil
	case "concurrent":
		return types.ScheduleOverlapPolicyConcurrent, nil
	case "cancelprevious", "cancel_previous":
		return types.ScheduleOverlapPolicyCancelPrevious, nil
	case "terminateprevious", "terminate_previous":
		return types.ScheduleOverlapPolicyTerminatePrevious, nil
	default:
		return 0, commoncli.Problem(fmt.Sprintf("Unknown overlap policy %q. Valid: SkipNew, Buffer, Concurrent, CancelPrevious, TerminatePrevious", s), nil)
	}
}

func parseCatchUpPolicy(s string) (types.ScheduleCatchUpPolicy, error) {
	switch strings.ToLower(s) {
	case "skip":
		return types.ScheduleCatchUpPolicySkip, nil
	case "one":
		return types.ScheduleCatchUpPolicyOne, nil
	case "all":
		return types.ScheduleCatchUpPolicyAll, nil
	default:
		return 0, commoncli.Problem(fmt.Sprintf("Unknown catch-up policy %q. Valid: Skip, One, All", s), nil)
	}
}

func printDescribeSchedule(resp *types.DescribeScheduleResponse) {
	fmt.Println("Schedule Configuration:")

	if spec := resp.GetSpec(); spec != nil {
		fmt.Printf("  Cron Expression:  %s\n", spec.CronExpression)
	}

	if action := resp.GetAction(); action != nil {
		if sw := action.StartWorkflow; sw != nil {
			if sw.WorkflowType != nil {
				fmt.Printf("  Workflow Type:    %s\n", sw.WorkflowType.Name)
			}
			if sw.TaskList != nil {
				fmt.Printf("  Task List:        %s\n", sw.TaskList.Name)
			}
		}
	}

	if policies := resp.GetPolicies(); policies != nil {
		fmt.Printf("  Overlap Policy:   %s\n", policies.OverlapPolicy)
		fmt.Printf("  Catch-up Policy:  %s\n", policies.CatchUpPolicy)
	}

	if state := resp.GetState(); state != nil {
		if state.Paused {
			fmt.Printf("  Status:           PAUSED\n")
			if pi := state.PauseInfo; pi != nil {
				fmt.Printf("  Pause Reason:     %s\n", pi.Reason)
				if pi.PausedBy != "" {
					fmt.Printf("  Paused By:        %s\n", pi.PausedBy)
				}
			}
		} else {
			fmt.Printf("  Status:           ACTIVE\n")
		}
	}

	if info := resp.GetInfo(); info != nil {
		fmt.Printf("  Total Runs:       %d\n", info.TotalRuns)
		if !info.LastRunTime.IsZero() {
			fmt.Printf("  Last Run:         %s\n", info.LastRunTime.Format(time.RFC3339))
		}
		if !info.NextRunTime.IsZero() {
			fmt.Printf("  Next Run:         %s\n", info.NextRunTime.Format(time.RFC3339))
		}
	}
}
