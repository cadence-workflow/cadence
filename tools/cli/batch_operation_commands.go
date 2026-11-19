package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/pborman/uuid"
	cli "github.com/urfave/cli/v2"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/common/commoncli"
)

// getBatchOperationsClient forces gRPC transport, since the batch API has
// no thrift equivalent, and returns the frontend client a batch start command needs.
func getBatchOperationsClient(c *cli.Context) (frontend.Client, error) {
	if c.IsSet(FlagTransport) && c.String(FlagTransport) != grpcTransport {
		return nil, commoncli.Problem("Batch commands require gRPC transport", fmt.Errorf("got --%s %q, the batch API has no thrift equivalent", FlagTransport, c.String(FlagTransport)))
	}
	if err := c.Set(FlagTransport, grpcTransport); err != nil {
		return nil, commoncli.Problem("Batch commands require gRPC transport but failed to set it", err)
	}
	if c.IsSet(FlagAddress) {
		fmt.Fprintf(getDeps(c).Progress(), "Note: batch commands always use gRPC; --%s must point at the gRPC port.\n", FlagAddress)
	}
	return getDeps(c).ServerFrontendClient(c)
}

// startBatchOperation does the shared work: flags, confirmation prompt, and
// calling StartBatchOperation. Each StartBatchXxx function just builds its
// own types.BatchOperation.
func startBatchOperation(c *cli.Context, frontendClient frontend.Client, operation *types.BatchOperation) error {
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	query, err := getRequiredOption(c, FlagListQuery)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	reason, err := getRequiredOption(c, FlagReason)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}

	ctx, cancel, err := newContext(c)
	if err != nil {
		return commoncli.Problem("Error in creating context:", err)
	}
	countResp, err := frontendClient.CountWorkflowExecutions(ctx, &types.CountWorkflowExecutionsRequest{
		Domain: domain,
		Query:  query,
	})
	cancel()
	if err != nil {
		return commoncli.Problem("Failed to count impacted workflows for starting a batch operation", err)
	}
	progress := getDeps(c).Progress()
	fmt.Fprintf(progress, "This batch operation will be operating on %v workflows.\n", countResp.GetCount())
	if !c.Bool(FlagYes) {
		reader := bufio.NewReader(getDeps(c).Input())
		for {
			fmt.Fprint(progress, "Please confirm[Yes/No]:")
			text, err := reader.ReadString('\n')
			if err != nil {
				return commoncli.Problem("Failed to get confirmation for starting a batch operation", err)
			}
			if strings.EqualFold(strings.TrimSpace(text), "yes") {
				break
			}
			fmt.Fprintln(progress, "Batch operation is not started")
			return nil
		}
	}

	batchID := c.String(FlagBatchID)
	if batchID == "" {
		batchID = uuid.NewRandom().String()
	}

	request := &types.StartBatchOperationRequest{
		Domain:    domain,
		BatchID:   batchID,
		Query:     query,
		Reason:    reason,
		Operation: operation,
		Identity:  getCurrentUserFromEnv(),
		Limits: &types.BatchOperationLimits{
			RPS:         int32(c.Int(FlagRPS)),
			Concurrency: int32(c.Int(FlagConcurrency)),
			PageSize:    int32(c.Int(FlagPageSize)),
		},
	}

	ctx, cancel, err = newContext(c)
	if err != nil {
		return commoncli.Problem("Error in creating context:", err)
	}
	defer cancel()

	resp, err := frontendClient.StartBatchOperation(ctx, request)
	if err != nil {
		return commoncli.Problem("Failed to start batch operation", err)
	}

	output := map[string]interface{}{
		"msg":     "batch operation is started",
		"batchID": resp.GetBatchID(),
	}
	prettyPrintJSONObject(getDeps(c).Output(), output)
	return nil
}

func StartBatchTerminate(c *cli.Context) error {
	frontendClient, err := getBatchOperationsClient(c)
	if err != nil {
		return err
	}
	return startBatchOperation(c, frontendClient, &types.BatchOperation{
		OperationType:           types.BatchOperationTypeTerminate.Ptr(),
		TerminateBatchOperation: &types.TerminateBatchOperation{},
	})
}

func StartBatchCancel(c *cli.Context) error {
	frontendClient, err := getBatchOperationsClient(c)
	if err != nil {
		return err
	}
	return startBatchOperation(c, frontendClient, &types.BatchOperation{
		OperationType:        types.BatchOperationTypeCancel.Ptr(),
		CancelBatchOperation: &types.CancelBatchOperation{},
	})
}

func StartBatchSignal(c *cli.Context) error {
	signalName, err := getRequiredOption(c, FlagSignalName)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	input := c.String(FlagInput)
	frontendClient, err := getBatchOperationsClient(c)
	if err != nil {
		return err
	}
	return startBatchOperation(c, frontendClient, &types.BatchOperation{
		OperationType: types.BatchOperationTypeSignal.Ptr(),
		SignalBatchOperation: &types.SignalBatchOperation{
			SignalName: signalName,
			Input:      []byte(input),
		},
	})
}

func StartBatchReplicate(c *cli.Context) error {
	sourceCluster, err := getRequiredOption(c, FlagSourceCluster)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	targetCluster, err := getRequiredOption(c, FlagTargetCluster)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	frontendClient, err := getBatchOperationsClient(c)
	if err != nil {
		return err
	}
	return startBatchOperation(c, frontendClient, &types.BatchOperation{
		OperationType: types.BatchOperationTypeReplicate.Ptr(),
		ReplicateBatchOperation: &types.ReplicateBatchOperation{
			SourceCluster: sourceCluster,
			TargetCluster: targetCluster,
		},
	})
}

func StartBatchRefresh(c *cli.Context) error {
	frontendClient, err := getBatchOperationsClient(c)
	if err != nil {
		return err
	}
	return startBatchOperation(c, frontendClient, &types.BatchOperation{
		OperationType:         types.BatchOperationTypeRefresh.Ptr(),
		RefreshBatchOperation: &types.RefreshBatchOperation{},
	})
}

var batchOperationStateMap = map[string]types.BatchOperationState{
	"running":   types.BatchOperationStateRunning,
	"completed": types.BatchOperationStateCompleted,
	"failed":    types.BatchOperationStateFailed,
	"stopped":   types.BatchOperationStateStopped,
}

func getBatchOperationState(stateStr string) (types.BatchOperationState, error) {
	if state, ok := batchOperationStateMap[strings.ToLower(stateStr)]; ok {
		return state, nil
	}
	return types.BatchOperationStateInvalid, commoncli.Problem(
		"option state is not one of allowed values [running, completed, failed, stopped]", nil)
}

// DescribeBatchOperation prints the status and progress of a single batch.
func DescribeBatchOperation(c *cli.Context) error {
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	batchID, err := getRequiredOption(c, FlagBatchID)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	frontendClient, err := getBatchOperationsClient(c)
	if err != nil {
		return err
	}

	ctx, cancel, err := newContext(c)
	if err != nil {
		return commoncli.Problem("Error in creating context:", err)
	}
	defer cancel()

	resp, err := frontendClient.DescribeBatchOperation(ctx, &types.DescribeBatchOperationRequest{
		Domain:  domain,
		BatchID: batchID,
	})
	if err != nil {
		return commoncli.Problem("Failed to describe batch operation", err)
	}

	output := map[string]interface{}{
		"batchID":   batchID,
		"state":     resp.GetState().String(),
		"reason":    resp.GetReason(),
		"startedBy": resp.GetStartedBy(),
		"startTime": resp.GetStartTime(),
		"closeTime": resp.GetCloseTime(),
		"progress":  resp.GetProgress(),
		"limits":    resp.GetLimits(),
	}
	prettyPrintJSONObject(getDeps(c).Output(), output)
	return nil
}

// ListBatchOperations lists the batch operations started for a domain.
func ListBatchOperations(c *cli.Context) error {
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	frontendClient, err := getBatchOperationsClient(c)
	if err != nil {
		return err
	}

	request := &types.ListBatchOperationsRequest{
		Domain:   domain,
		PageSize: int32(c.Int(FlagPageSize)),
	}
	if stateStr := c.String(FlagBatchOperationState); stateStr != "" {
		state, err := getBatchOperationState(stateStr)
		if err != nil {
			return err
		}
		request.State = state
	}

	ctx, cancel, err := newContext(c)
	if err != nil {
		return commoncli.Problem("Error in creating context:", err)
	}
	defer cancel()

	found := false
	for {
		resp, err := frontendClient.ListBatchOperations(ctx, request)
		if err != nil {
			return commoncli.Problem("Failed to list batch operations", err)
		}
		for _, entry := range resp.GetBatchOperations() {
			found = true
			fmt.Printf("  %-40s  %-10s  %-20s\n", entry.BatchID, entry.State.String(), entry.StartedBy)
		}
		if len(resp.GetNextPageToken()) == 0 {
			break
		}
		request.NextPageToken = resp.GetNextPageToken()
	}

	if !found {
		fmt.Println("No batch operations found.")
	}
	return nil
}

// StopBatchOperation terminates a running batch operation.
func StopBatchOperation(c *cli.Context) error {
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	batchID, err := getRequiredOption(c, FlagBatchID)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	frontendClient, err := getBatchOperationsClient(c)
	if err != nil {
		return err
	}

	ctx, cancel, err := newContext(c)
	if err != nil {
		return commoncli.Problem("Error in creating context:", err)
	}
	defer cancel()

	_, err = frontendClient.StopBatchOperation(ctx, &types.StopBatchOperationRequest{
		Domain:   domain,
		BatchID:  batchID,
		Reason:   c.String(FlagReason),
		Identity: getCurrentUserFromEnv(),
	})
	if err != nil {
		return commoncli.Problem("Failed to stop batch operation", err)
	}

	output := map[string]interface{}{
		"msg":     "batch operation is stopped",
		"batchID": batchID,
	}
	prettyPrintJSONObject(getDeps(c).Output(), output)
	return nil
}

// UpdateBatchOperation changes the RPS and/or concurrency of a running batch.
func UpdateBatchOperation(c *cli.Context) error {
	domain, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	batchID, err := getRequiredOption(c, FlagBatchID)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	if !c.IsSet(FlagRPS) && !c.IsSet(FlagConcurrency) {
		return commoncli.Problem("At least one of --rps or --concurrency must be set", nil)
	}
	frontendClient, err := getBatchOperationsClient(c)
	if err != nil {
		return err
	}

	ctx, cancel, err := newContext(c)
	if err != nil {
		return commoncli.Problem("Error in creating context:", err)
	}
	defer cancel()

	_, err = frontendClient.UpdateBatchOperation(ctx, &types.UpdateBatchOperationRequest{
		Domain:      domain,
		BatchID:     batchID,
		RPS:         int32(c.Int(FlagRPS)),
		Concurrency: int32(c.Int(FlagConcurrency)),
		Identity:    getCurrentUserFromEnv(),
	})
	if err != nil {
		return commoncli.Problem("Failed to update batch operation", err)
	}

	output := map[string]interface{}{
		"msg":     "batch operation is updated",
		"batchID": batchID,
	}
	prettyPrintJSONObject(getDeps(c).Output(), output)
	return nil
}
