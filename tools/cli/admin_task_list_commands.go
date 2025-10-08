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

package cli

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/common/commoncli"
)

type (
	TaskListRow struct {
		Name                string `header:"Task List Name"`
		ActivityPollerCount int    `header:"Activity Poller Count"`
		DecisionPollerCount int    `header:"Decision Poller Count"`
	}
	TaskListStatusRow struct {
		Type        string  `header:"Type"`
		PollerCount int     `header:"PollerCount"`
		ReadLevel   int64   `header:"Read Level"`
		AckLevel    int64   `header:"Ack Level"`
		Backlog     int64   `header:"Backlog"`
		RPS         float64 `header:"RPS"`
		StartID     int64   `header:"Lease Start TaskID"`
		EndID       int64   `header:"Lease End TaskID"`
	}
	TaskListPartitionConfigRow struct {
		Type            string                           `header:"Type"`
		Version         int64                            `header:"Version"`
		ReadPartitions  map[int]*types.TaskListPartition `header:"Read Partitions"`
		WritePartitions map[int]*types.TaskListPartition `header:"Write Partitions"`
	}
)

// AdminDescribeTaskList displays poller and status information of task list.
func AdminDescribeTaskList(c *cli.Context) error {
	frontendClient, err := getDeps(c).ServerFrontendClient(c)
	if err != nil {
		return err
	}
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	taskList, err := getRequiredOption(c, FlagTaskList)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	taskListTypes, err := getTaskListTypes(c)
	if err != nil {
		return err
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context:", err)
	}
	responses := make(map[types.TaskListType]*types.DescribeTaskListResponse)
	for _, tlType := range taskListTypes {
		request := &types.DescribeTaskListRequest{
			Domain:                domain,
			TaskList:              &types.TaskList{Name: taskList},
			TaskListType:          tlType.Ptr(),
			IncludeTaskListStatus: true,
		}

		response, err := frontendClient.DescribeTaskList(ctx, request)
		if err != nil {
			return commoncli.Problem("Operation DescribeTaskList failed for type: "+tlType.String(), err)
		}
		responses[tlType] = response
	}
	if c.String(FlagFormat) == formatJSON {
		prettyPrintJSONObject(getDeps(c).Output(), responses)
		return nil
	}

	if err := printTaskListStatus(getDeps(c).Output(), responses); err != nil {
		return fmt.Errorf("failed to print task list status: %w", err)
	}
	getDeps(c).Output().Write([]byte("\n"))
	if err := printTaskListPartitionConfig(getDeps(c).Output(), responses); err != nil {
		return fmt.Errorf("failed to print task list partition config: %w", err)
	}
	getDeps(c).Output().Write([]byte("\n"))

	return nil
}

// AdminListTaskList displays all task lists under a domain.
func AdminListTaskList(c *cli.Context) error {
	frontendClient, err := getDeps(c).ServerFrontendClient(c)
	if err != nil {
		return err
	}
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	request := &types.GetTaskListsByDomainRequest{
		Domain: domain,
	}

	response, err := frontendClient.GetTaskListsByDomain(ctx, request)
	if err != nil {
		return commoncli.Problem("Operation GetTaskListByDomain failed.", err)
	}

	if c.String(FlagFormat) == formatJSON {
		prettyPrintJSONObject(getDeps(c).Output(), response)
		return nil
	}

	_, _ = fmt.Fprintln(getDeps(c).Output(), "Task Lists for domain ", domain, ":")
	tlByName := make(map[string]TaskListRow)
	for name, taskList := range response.GetDecisionTaskListMap() {
		row := tlByName[name]
		row.Name = name
		row.DecisionPollerCount = len(taskList.GetPollers())
		tlByName[name] = row
	}
	for name, taskList := range response.GetActivityTaskListMap() {
		row := tlByName[name]
		row.Name = name
		row.ActivityPollerCount = len(taskList.GetPollers())
		tlByName[name] = row
	}
	table := slices.Collect(maps.Values(tlByName))
	slices.SortFunc(table, func(a, b TaskListRow) int {
		return strings.Compare(a.Name, b.Name)
	})
	return RenderTable(os.Stdout, table, RenderOptions{Color: true, Border: true})
}

func printTaskListStatus(w io.Writer, responses map[types.TaskListType]*types.DescribeTaskListResponse) error {
	var table []TaskListStatusRow
	for tlType, response := range responses {
		taskListStatus := response.TaskListStatus
		table = append(table, TaskListStatusRow{
			Type:        tlType.String(),
			PollerCount: len(response.Pollers),
			ReadLevel:   taskListStatus.GetReadLevel(),
			AckLevel:    taskListStatus.GetAckLevel(),
			Backlog:     taskListStatus.GetBacklogCountHint(),
			RPS:         taskListStatus.GetRatePerSecond(),
			StartID:     taskListStatus.GetTaskIDBlock().GetStartID(),
			EndID:       taskListStatus.GetTaskIDBlock().GetEndID(),
		})
	}
	slices.SortFunc(table, func(a, b TaskListStatusRow) int {
		return strings.Compare(a.Type, b.Type)
	})
	return RenderTable(w, table, RenderOptions{Color: true})
}

func printTaskListPartitionConfig(w io.Writer, responses map[types.TaskListType]*types.DescribeTaskListResponse) error {
	var table []TaskListPartitionConfigRow
	for tlType, response := range responses {
		config := response.PartitionConfig
		if config == nil {
			config = &types.TaskListPartitionConfig{
				Version:         0,
				ReadPartitions:  createPartitions(1),
				WritePartitions: createPartitions(1),
			}
		}
		table = append(table, TaskListPartitionConfigRow{
			Type:            tlType.String(),
			Version:         config.Version,
			ReadPartitions:  config.ReadPartitions,
			WritePartitions: config.WritePartitions,
		})
	}
	slices.SortFunc(table, func(a, b TaskListPartitionConfigRow) int {
		return strings.Compare(a.Type, b.Type)
	})
	return RenderTable(w, table, RenderOptions{Color: true})
}

func AdminUpdateTaskListPartitionConfig(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}
	frontendClient, err := getDeps(c).ServerFrontendClient(c)
	if err != nil {
		return err
	}
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	taskList, err := getRequiredOption(c, FlagTaskList)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	force := c.Bool(FlagForce)
	taskListTypes, err := getTaskListTypes(c)
	if err != nil {
		return err
	}
	numReadPartitions, err := getRequiredIntOption(c, FlagNumReadPartitions)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	numWritePartitions, err := getRequiredIntOption(c, FlagNumWritePartitions)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context:", err)
	}
	cfg := &types.TaskListPartitionConfig{
		ReadPartitions:  createPartitions(numReadPartitions),
		WritePartitions: createPartitions(numWritePartitions),
	}
	tl := &types.TaskList{Name: taskList, Kind: types.TaskListKindNormal.Ptr()}
	if !force {
		hasPollers := false
		var errors []error
		for _, tlType := range taskListTypes {
			typeHasPollers, typeErr := validateChange(ctx, frontendClient, domain, tl, tlType.Ptr(), cfg)
			if typeErr != nil {
				errors = append(errors, fmt.Errorf("%s:%s failed validation: %w", tl.Name, tlType, typeErr))
			}
			hasPollers = typeHasPollers || hasPollers
		}
		if len(errors) > 0 {
			return commoncli.Problem("Potentially unsafe operation. Specify '--force' to proceed anyway", multierr.Combine(errors...))
		}
		if !hasPollers {
			return commoncli.Problem(fmt.Sprintf("Operation is safe but %s has no pollers of the specified types. Is the name correct? Specify '--force' to proceed anyway", tl.Name), nil)
		}
	}
	for _, tlType := range taskListTypes {
		_, err = adminClient.UpdateTaskListPartitionConfig(ctx, &types.UpdateTaskListPartitionConfigRequest{
			Domain:          domain,
			TaskList:        tl,
			TaskListType:    tlType.Ptr(),
			PartitionConfig: cfg,
		})
		if err != nil {
			return commoncli.Problem("Operation UpdateTaskListPartitionConfig failed for type: "+tlType.String(), err)
		}
		_, _ = fmt.Fprintln(getDeps(c).Output(), "Successfully updated ", tl.Name, ":", tlType)
	}
	return nil
}

func validateChange(ctx context.Context, client frontend.Client, domain string, tl *types.TaskList, tlt *types.TaskListType, newCfg *types.TaskListPartitionConfig) (bool, error) {
	description, err := client.DescribeTaskList(ctx, &types.DescribeTaskListRequest{
		Domain:       domain,
		TaskList:     tl,
		TaskListType: tlt,
	})
	if err != nil {
		return false, fmt.Errorf("DescribeTaskList failed: %w", err)
	}
	// Illegal operations are rejected by the server (read < write), but unsafe ones are still allowed
	if description.PartitionConfig != nil {
		oldCfg := description.PartitionConfig
		// Ensure they're not removing active write partitions
		if len(newCfg.ReadPartitions) < len(oldCfg.WritePartitions) {
			return false, fmt.Errorf("remove write partitions, then read partitions. Removing an active write partition risks losing tasks. Proposed read count is less than current write count (%d < %d)", len(newCfg.ReadPartitions), len(oldCfg.WritePartitions))
		}
		// Ensure removed read partitions are drained
		for i := len(newCfg.ReadPartitions); i < len(oldCfg.ReadPartitions); i++ {
			partition, err := client.DescribeTaskList(ctx, &types.DescribeTaskListRequest{
				Domain:                domain,
				TaskList:              &types.TaskList{Name: getPartitionTaskListName(tl.Name, i), Kind: tl.Kind},
				TaskListType:          tlt,
				IncludeTaskListStatus: true,
			})
			if err != nil {
				return false, fmt.Errorf("DescribeTaskList failed for partition %d: %w", i, err)
			}
			if partition.TaskListStatus.BacklogCountHint != 0 {
				return false, fmt.Errorf("partition %d still has %d tasks remaining", i, partition.TaskListStatus.BacklogCountHint)
			}
		}
	}
	// If it's otherwise valid but there are no pollers, they might have mistyped the name
	return len(description.Pollers) > 0, nil
}

func createPartitions(num int) map[int]*types.TaskListPartition {
	result := make(map[int]*types.TaskListPartition, num)
	for i := 0; i < num; i++ {
		result[i] = &types.TaskListPartition{}
	}
	return result
}

func getPartitionTaskListName(root string, partition int) string {
	if partition <= 0 {
		return root
	}
	return fmt.Sprintf("%v%v/%v", constants.ReservedTaskListPrefix, root, partition)
}

func getTaskListTypes(c *cli.Context) ([]types.TaskListType, error) {
	var taskListTypes []types.TaskListType
	if strings.ToLower(c.String(FlagTaskListType)) == "activity" {
		taskListTypes = []types.TaskListType{types.TaskListTypeActivity}
	} else if strings.ToLower(c.String(FlagTaskListType)) == "decision" {
		taskListTypes = []types.TaskListType{types.TaskListTypeDecision}
	} else if c.String(FlagTaskListType) == "" {
		taskListTypes = []types.TaskListType{types.TaskListTypeActivity, types.TaskListTypeDecision}
	} else {
		return nil, commoncli.Problem("Invalid task list type: valid types are 'activity', 'decision', or empty (both)", nil)
	}
	return taskListTypes, nil
}
