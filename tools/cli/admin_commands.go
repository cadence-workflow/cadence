// Copyright (c) 2017-2020 Uber Technologies Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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
	"io/ioutil"
	"strconv"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
	"github.com/uber/cadence/tools/common/commoncli"
)

const (
	tableRenderSize = 10
	remoteHint      = " (Hint: use --remote flag to run this operation via the server API instead of direct DB access)"
)

// AdminShowWorkflow shows history
func AdminShowWorkflow(c *cli.Context) error {
	tid := c.String(FlagTreeID)
	bid := c.String(FlagBranchID)
	sid := c.Int(FlagShardID)
	minEventID := c.Int64(FlagMinEventID)
	maxEventID := c.Int64(FlagMaxEventID)
	outputFileName := c.String(FlagOutputFilename)
	domainName := c.String(FlagDomain)
	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	serializer := persistence.NewPayloadSerializer()
	var history []*persistence.DataBlob
	if len(tid) != 0 {
		thriftrwEncoder := codec.NewThriftRWEncoder()
		histV2, err := getDeps(c).initializeHistoryManager(c)
		if err != nil {
			return commoncli.Problem("Error in Admin delete WF: ", err)
		}
		branchToken, err := thriftrwEncoder.Encode(&shared.HistoryBranch{
			TreeID:   &tid,
			BranchID: &bid,
		})
		if err != nil {
			return commoncli.Problem("encoding branch token err", err)
		}
		resp, err := histV2.ReadRawHistoryBranch(ctx, &persistence.ReadHistoryBranchRequest{
			BranchToken: branchToken,
			MinEventID:  minEventID,
			MaxEventID:  maxEventID,
			PageSize:    int(maxEventID - minEventID + 1),
			ShardID:     &sid,
			DomainName:  domainName,
		})
		if err != nil {
			return commoncli.Problem("ReadHistoryBranch err", err)
		}

		history = resp.HistoryEventBlobs
	} else {
		return commoncli.Problem("need to specify TreeID/BranchID/ShardID", nil)
	}

	if len(history) == 0 {
		return commoncli.Problem("no events", nil)
	}
	allEvents := &shared.History{}
	totalSize := 0
	for idx, b := range history {
		totalSize += len(b.Data)
		fmt.Printf("======== batch %v, blob len: %v ======\n", idx+1, len(b.Data))
		internalHistoryBatch, err := serializer.DeserializeBatchEvents(b)
		if err != nil {
			return commoncli.Problem("DeserializeBatchEvents err", err)
		}
		historyBatch := thrift.FromHistoryEventArray(internalHistoryBatch)
		allEvents.Events = append(allEvents.Events, historyBatch...)
		for _, e := range historyBatch {
			jsonstr, err := json.Marshal(e)
			if err != nil {
				return commoncli.Problem("json.Marshal err", err)
			}
			fmt.Println(string(jsonstr))
		}
	}
	fmt.Printf("======== total batches %v, total blob len: %v ======\n", len(history), totalSize)

	if outputFileName != "" {
		data, err := json.Marshal(allEvents.Events)
		if err != nil {
			return commoncli.Problem("Failed to serialize history data.", err)
		}
		if err := ioutil.WriteFile(outputFileName, data, 0666); err != nil {
			return commoncli.Problem("Failed to export history data file.", err)
		}
	}
	return nil
}

// AdminDescribeWorkflow describe a new workflow execution for admin
func AdminDescribeWorkflow(c *cli.Context) error {

	resp, err := describeMutableState(c)
	if err != nil {
		return err
	}
	prettyPrintJSONObject(getDeps(c).Output(), resp)

	if resp != nil {
		msStr := resp.GetMutableStateInDatabase()
		ms := persistence.WorkflowMutableState{}
		err := json.Unmarshal([]byte(msStr), &ms)
		if err != nil {
			return commoncli.Problem("json.Unmarshal err", err)
		}
		currentBranchToken := ms.ExecutionInfo.BranchToken
		if ms.VersionHistories != nil {
			// if VersionHistories is set, then all branch infos are stored in VersionHistories
			currentVersionHistory, err := ms.VersionHistories.GetCurrentVersionHistory()
			if err != nil {
				return commoncli.Problem("ms.VersionHistories.GetCurrentVersionHistory err", err)
			}
			currentBranchToken = currentVersionHistory.GetBranchToken()
		}

		branchInfo := shared.HistoryBranch{}
		thriftrwEncoder := codec.NewThriftRWEncoder()
		err = thriftrwEncoder.Decode(currentBranchToken, &branchInfo)
		if err != nil {
			return commoncli.Problem("thriftrwEncoder.Decode err", err)
		}
		prettyPrintJSONObject(getDeps(c).Output(), branchInfo)
		if ms.ExecutionInfo.AutoResetPoints != nil {
			getDeps(c).Output().Write([]byte("auto-reset-points:"))
			for _, p := range ms.ExecutionInfo.AutoResetPoints.Points {
				createT := time.Unix(0, p.GetCreatedTimeNano())
				expireT := time.Unix(0, p.GetExpiringTimeNano())
				getDeps(c).Output().Write([]byte(fmt.Sprintln(p.GetBinaryChecksum(), p.GetRunID(), p.GetFirstDecisionCompletedID(), p.GetResettable(), createT, expireT)))
			}
		}
	}
	return nil
}

func describeMutableState(c *cli.Context) (*types.AdminDescribeWorkflowExecutionResponse, error) {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return nil, err
	}

	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return nil, commoncli.Problem("Required flag not found", err)
	}
	wid, err := getRequiredOption(c, FlagWorkflowID)
	if err != nil {
		return nil, commoncli.Problem("Required flag not found", err)
	}
	rid := c.String(FlagRunID)

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return nil, commoncli.Problem("Error in creating context: ", err)
	}
	resp, err := adminClient.DescribeWorkflowExecution(
		ctx,
		&types.AdminDescribeWorkflowExecutionRequest{
			Domain: domain,
			Execution: &types.WorkflowExecution{
				WorkflowID: wid,
				RunID:      rid,
			},
		},
	)
	if err != nil {
		return nil, commoncli.Problem("Get workflow mutableState failed", err)
	}
	return resp, nil
}

// AdminMaintainCorruptWorkflow deletes workflow from DB if it's corrupt
func AdminMaintainCorruptWorkflow(c *cli.Context) error {
	domainName, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	workflowID := c.String(FlagWorkflowID)
	runID := c.String(FlagRunID)
	skipErrors := c.Bool(FlagSkipErrorMode)
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	request := &types.AdminMaintainWorkflowRequest{
		Domain: domainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
		SkipErrors: skipErrors,
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	_, err = adminClient.MaintainCorruptWorkflow(ctx, request)
	if err != nil {
		return commoncli.Problem("Operation AdminMaintainCorruptWorkflow failed.", err)
	}

	return err
}

// AdminDeleteWorkflow delete a workflow execution for admin
func AdminDeleteWorkflow(c *cli.Context) error {
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	wid, err := getRequiredOption(c, FlagWorkflowID)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	rid := c.String(FlagRunID)
	remote := c.Bool(FlagRemote)
	skipError := c.Bool(FlagSkipErrorMode)

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	// With remote flag, we run the command on the server side using existing APIs
	// Without remote, commands are run directly through some DB clients. This is
	// useful if server is down somehow. However, we only support couple DB clients
	// currently. If the server side hosts working, remote is a cleaner approach
	if remote {
		adminClient, err := getDeps(c).ServerAdminClient(c)
		if err != nil {
			return err
		}
		request := &types.AdminDeleteWorkflowRequest{
			Domain: domain,
			Execution: &types.WorkflowExecution{
				WorkflowID: wid,
				RunID:      rid,
			},
			SkipErrors: skipError,
		}
		_, err = adminClient.DeleteWorkflow(ctx, request)
		if err != nil {
			return commoncli.Problem("Operation AdminMaintainCorruptWorkflow failed.", err)
		}
		return nil
	}

	resp, err := describeMutableState(c)
	if err != nil {
		return commoncli.Problem("Failed to get workflow mutable state"+remoteHint, err)
	}
	msStr := resp.GetMutableStateInDatabase()
	ms := persistence.WorkflowMutableState{}
	err = json.Unmarshal([]byte(msStr), &ms)
	if err != nil {
		return commoncli.Problem("json.Unmarshal err", err)
	}
	domainID := ms.ExecutionInfo.DomainID

	shardID := resp.GetShardID()
	shardIDInt, err := strconv.Atoi(shardID)
	if err != nil {
		return commoncli.Problem("strconv.Atoi(shardID) err", err)
	}
	histV2, err := getDeps(c).initializeHistoryManager(c)
	if err != nil {
		return commoncli.Problem("Unable to initialize history manager"+remoteHint, err)
	}
	defer histV2.Close()
	exeStore, err := getDeps(c).initializeExecutionManager(c, shardIDInt)
	if err != nil {
		return commoncli.Problem("Unable to initialize execution manager"+remoteHint, err)
	}
	branchInfo := shared.HistoryBranch{}
	thriftrwEncoder := codec.NewThriftRWEncoder()
	branchTokens := [][]byte{ms.ExecutionInfo.BranchToken}
	if ms.VersionHistories != nil {
		// if VersionHistories is set, then all branch infos are stored in VersionHistories
		branchTokens = [][]byte{}
		for _, versionHistory := range ms.VersionHistories.ToInternalType().Histories {
			branchTokens = append(branchTokens, versionHistory.BranchToken)
		}
	}

	for _, branchToken := range branchTokens {
		err = thriftrwEncoder.Decode(branchToken, &branchInfo)
		if err != nil {
			return commoncli.Problem("thriftrwEncoder.Decode err", err)
		}
		fmt.Println("deleting history events for ...")
		prettyPrintJSONObject(getDeps(c).Output(), branchInfo)
		err = histV2.DeleteHistoryBranch(ctx, &persistence.DeleteHistoryBranchRequest{
			BranchToken: branchToken,
			ShardID:     &shardIDInt,
			DomainName:  domain,
		})
		if err != nil {
			if skipError {
				fmt.Println("failed to delete history, ", err)
			} else {
				return commoncli.Problem("DeleteHistoryBranch err"+remoteHint, err)
			}
		}
	}

	req := &persistence.DeleteWorkflowExecutionRequest{
		DomainID:   domainID,
		WorkflowID: wid,
		RunID:      rid,
		DomainName: domain,
	}

	err = exeStore.DeleteWorkflowExecution(ctx, req)
	if err != nil {
		if skipError {
			fmt.Println("delete mutableState row failed, ", err)
		} else {
			return commoncli.Problem("delete mutableState row failed"+remoteHint, err)
		}
	}
	fmt.Println("delete mutableState row successfully")

	deleteCurrentReq := &persistence.DeleteCurrentWorkflowExecutionRequest{
		DomainID:   domainID,
		WorkflowID: wid,
		RunID:      rid,
	}

	err = exeStore.DeleteCurrentWorkflowExecution(ctx, deleteCurrentReq)
	if err != nil {
		if skipError {
			fmt.Println("delete current row failed, ", err)
		} else {
			return commoncli.Problem("delete current row failed"+remoteHint, err)
		}
	}
	fmt.Println("delete current row successfully")
	return nil
}

// AdminGetDomainIDOrName map domain
func AdminGetDomainIDOrName(c *cli.Context) error {
	domainID := c.String(FlagDomainID)
	domainName := c.String(FlagDomain)

	if (len(domainID) == 0 && len(domainName) == 0) || (len(domainID) != 0 && len(domainName) != 0) {
		return commoncli.Problem("Need either domainName or domainID", nil)
	}

	domainManager, err := getDeps(c).initializeDomainManager(c)
	if err != nil {
		return commoncli.Problem("Error in Admin delete WF: ", err)
	}
	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}

	output := getDeps(c).Output()
	if len(domainID) > 0 {
		res, err := domainManager.GetDomain(ctx, &persistence.GetDomainRequest{ID: domainID})
		if err != nil {
			return commoncli.Problem("GetDomain error", err)
		}
		fmt.Fprintf(output, "domainName for domainID %v is %v\n", domainID, res.Info.Name)
	} else {
		res, err := domainManager.GetDomain(ctx, &persistence.GetDomainRequest{Name: domainName})
		if err != nil {
			return commoncli.Problem("GetDomain error", err)
		}
		fmt.Fprintf(output, "domainID for domainName %v is %v\n", domainName, res.Info.ID)
	}
	return nil
}

// AdminGetShardID get shardID
func AdminGetShardID(c *cli.Context) error {
	wid, err := getRequiredOption(c, FlagWorkflowID)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	numberOfShards := c.Int(FlagNumberOfShards)

	if numberOfShards <= 0 {
		return commoncli.Problem("numberOfShards is required", nil)
	}
	shardID := common.WorkflowIDToHistoryShard(wid, numberOfShards)

	fmt.Fprintf(getDeps(c).Output(), "ShardID for workflowID: %v is %v\n", wid, shardID)
	return nil
}

// AdminRemoveTask describes history host
func AdminRemoveTask(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	shardID, err := getRequiredIntOption(c, FlagShardID)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	taskID, err := getRequiredInt64Option(c, FlagTaskID)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	typeID, err := getRequiredIntOption(c, FlagTaskType)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	var visibilityTimestamp int64
	if constants.TaskType(typeID) == constants.TaskTypeTimer {
		visibilityTimestamp, err = getRequiredInt64Option(c, FlagTaskVisibilityTimestamp)
		if err != nil {
			return commoncli.Problem("Required flag not found", err)
		}
	}
	var clusterName string

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	req := &types.RemoveTaskRequest{
		ShardID:             int32(shardID),
		Type:                common.Int32Ptr(int32(typeID)),
		TaskID:              taskID,
		VisibilityTimestamp: common.Int64Ptr(visibilityTimestamp),
		ClusterName:         clusterName,
	}

	err = adminClient.RemoveTask(ctx, req)
	if err != nil {
		return commoncli.Problem("Remove task has failed", err)
	}
	return nil
}

// AdminDescribeShard describes shard by shard id
func AdminDescribeShard(c *cli.Context) error {
	sid, err := getRequiredIntOption(c, FlagShardID)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	shardManager, err := getDeps(c).initializeShardManager(c)
	if err != nil {
		return commoncli.Problem("Error in describe shard: ", err)
	}
	getShardReq := &persistence.GetShardRequest{ShardID: sid}
	resp, err := shardManager.GetShard(ctx, getShardReq)
	if err != nil {
		return commoncli.Problem("Failed to describe shard.", err)
	}

	prettyPrintJSONObject(getDeps(c).Output(), resp)
	return nil
}

// AdminSetShardRangeID set shard rangeID by shard id
func AdminSetShardRangeID(c *cli.Context) error {
	sid, err := getRequiredIntOption(c, FlagShardID)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	rid, err := getRequiredInt64Option(c, FlagRangeID)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	shardManager, err := getDeps(c).initializeShardManager(c)
	if err != nil {
		return commoncli.Problem("Error in Admin SetShardRangeID: ", err)
	}
	getShardResp, err := shardManager.GetShard(ctx, &persistence.GetShardRequest{ShardID: sid})
	if err != nil {
		return commoncli.Problem("Failed to get shardInfo.", err)
	}

	previousRangeID := getShardResp.ShardInfo.RangeID
	updatedShardInfo := getShardResp.ShardInfo
	updatedShardInfo.RangeID = rid
	updatedShardInfo.StolenSinceRenew++
	updatedShardInfo.Owner = ""
	updatedShardInfo.UpdatedAt = time.Now()

	err = shardManager.UpdateShard(ctx, &persistence.UpdateShardRequest{
		PreviousRangeID: previousRangeID,
		ShardInfo:       updatedShardInfo,
	})
	if err != nil {
		return commoncli.Problem("Failed to reset shard rangeID.", err)
	}

	fmt.Fprintf(getDeps(c).Output(), "Successfully updated rangeID from %v to %v for shard %v.\n", previousRangeID, rid, sid)
	return nil
}

// AdminCloseShard closes shard by shard id
func AdminCloseShard(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}
	sid, err := getRequiredIntOption(c, FlagShardID)
	if err != nil {
		return commoncli.Problem("Required option not found", err)
	}
	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context", err)
	}
	req := &types.CloseShardRequest{}
	req.ShardID = int32(sid)

	err = adminClient.CloseShard(ctx, req)
	if err != nil {
		return commoncli.Problem("Close shard task has failed", err)
	}
	return nil
}

type ShardRow struct {
	ShardID  int32  `header:"ShardID"`
	Identity string `header:"Identity"`
}

// AdminDescribeShardDistribution describes shard distribution
func AdminDescribeShardDistribution(c *cli.Context) error {
	output := getDeps(c).Output()

	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context", err)
	}
	req := &types.DescribeShardDistributionRequest{
		PageSize: int32(c.Int(FlagPageSize)),
		PageID:   int32(c.Int(FlagPageID)),
	}

	resp, err := adminClient.DescribeShardDistribution(ctx, req)
	if err != nil {
		return commoncli.Problem("Shard list failed", err)
	}

	fmt.Fprintf(output, "Total Number of Shards: %d\n", resp.NumberOfShards)
	fmt.Fprintf(output, "Number of Shards Returned: %d\n", len(resp.Shards))

	if len(resp.Shards) == 0 {
		return nil
	}

	table := []ShardRow{}
	opts := RenderOptions{DefaultTemplate: templateTable, Color: true}
	outputPageSize := tableRenderSize
	for shardID, identity := range resp.Shards {
		if outputPageSize == 0 {
			if err := Render(c, table, opts); err != nil {
				return fmt.Errorf("error rendering: %w", err)
			}
			table = []ShardRow{}
			if !showNextPage(output) {
				break
			}
			outputPageSize = tableRenderSize
		}
		table = append(table, ShardRow{ShardID: shardID, Identity: identity})
		outputPageSize--
	}
	// output the remaining rows
	return Render(c, table, opts)
}

// AdminDescribeHistoryHost describes history host
func AdminDescribeHistoryHost(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	wid := c.String(FlagWorkflowID)
	sid := c.Int(FlagShardID)
	addr := c.String(FlagHistoryAddress)
	printFully := c.Bool(FlagPrintFullyDetail)

	if len(wid) == 0 && !c.IsSet(FlagShardID) && len(addr) == 0 {
		return commoncli.Problem("at least one of them is required to provide to lookup host: workflowID, shardID and host address", nil)
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context", err)
	}
	req := &types.DescribeHistoryHostRequest{}
	if len(wid) > 0 {
		req.ExecutionForHost = &types.WorkflowExecution{WorkflowID: wid}
	}
	if c.IsSet(FlagShardID) {
		req.ShardIDForHost = common.Int32Ptr(int32(sid))
	}
	if len(addr) > 0 {
		req.HostAddress = common.StringPtr(addr)
	}

	resp, err := adminClient.DescribeHistoryHost(ctx, req)
	if err != nil {
		return commoncli.Problem("Describe history host failed", err)
	}

	if !printFully {
		resp.ShardIDs = nil
	}
	prettyPrintJSONObject(getDeps(c).Output(), resp)
	return nil
}

// AdminRefreshWorkflowTasks refreshes all the tasks of a workflow
func AdminRefreshWorkflowTasks(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	wid, err := getRequiredOption(c, FlagWorkflowID)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	rid := c.String(FlagRunID)

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	err = adminClient.RefreshWorkflowTasks(ctx, &types.RefreshWorkflowTasksRequest{
		Domain: domain,
		Execution: &types.WorkflowExecution{
			WorkflowID: wid,
			RunID:      rid,
		},
	})
	if err != nil {
		return commoncli.Problem("Refresh workflow task failed", err)
	}
	fmt.Fprintln(getDeps(c).Output(), "Refresh workflow task succeeded.")
	return nil
}

// AdminResetQueue resets task processing queue states
func AdminResetQueue(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	shardID, err := getRequiredIntOption(c, FlagShardID)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	clusterName, err := getRequiredOption(c, FlagCluster)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	typeID, err := getRequiredIntOption(c, FlagQueueType)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	req := &types.ResetQueueRequest{
		ShardID:     int32(shardID),
		ClusterName: clusterName,
		Type:        common.Int32Ptr(int32(typeID)),
	}

	err = adminClient.ResetQueue(ctx, req)
	if err != nil {
		return commoncli.Problem("Failed to reset queue", err)
	}

	fmt.Fprintln(getDeps(c).Output(), "Reset queue state succeeded")
	return nil
}

// AdminDescribeQueue describes task processing queue states
func AdminDescribeQueue(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	shardID, err := getRequiredIntOption(c, FlagShardID)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	clusterName, err := getRequiredOption(c, FlagCluster)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	typeID, err := getRequiredIntOption(c, FlagQueueType)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	req := &types.DescribeQueueRequest{
		ShardID:     int32(shardID),
		ClusterName: clusterName,
		Type:        common.Int32Ptr(int32(typeID)),
	}

	resp, err := adminClient.DescribeQueue(ctx, req)
	if err != nil {
		return commoncli.Problem("Failed to describe queue", err)
	}

	output := getDeps(c).Output()
	for _, state := range resp.ProcessingQueueStates {
		fmt.Fprintln(output, state)
	}
	return nil
}
