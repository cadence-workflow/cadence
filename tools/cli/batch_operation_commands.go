package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/pborman/uuid"
	cli "github.com/urfave/cli/v2"

	"github.com/uber/cadence/client/batch"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/common/commoncli"
)

// getBatchOperationsClients forces gRPC transport, since the batch API has
// no thrift equivalent, and returns the clients a batch start command needs.
func getBatchOperationsClients(c *cli.Context) (batch.Client, frontend.Client, error) {
	if c.IsSet(FlagTransport) && c.String(FlagTransport) != grpcTransport {
		return nil, nil, commoncli.Problem("Batch commands require gRPC transport", fmt.Errorf("got --%s %q, the batch API has no thrift equivalent", FlagTransport, c.String(FlagTransport)))
	}
	if err := c.Set(FlagTransport, grpcTransport); err != nil {
		return nil, nil, commoncli.Problem("Batch commands require gRPC transport but failed to set it", err)
	}
	if c.IsSet(FlagAddress) {
		fmt.Fprintf(getDeps(c).Progress(), "Note: batch commands always use gRPC; --%s must point at the gRPC port.\n", FlagAddress)
	}
	batchClient, err := getDeps(c).ServerBatchClient(c)
	if err != nil {
		return nil, nil, err
	}
	frontendClient, err := getDeps(c).ServerFrontendClient(c)
	if err != nil {
		return nil, nil, err
	}
	return batchClient, frontendClient, nil
}

// startBatchOperation does the shared work: flags, confirmation prompt, and
// calling StartBatchOperation. Each StartBatchXxx function just builds its
// own types.BatchOperation.
func startBatchOperation(c *cli.Context, batchClient batch.Client, frontendClient frontend.Client, operation *types.BatchOperation) error {
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

	resp, err := batchClient.StartBatchOperation(ctx, request)
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
	batchClient, frontendClient, err := getBatchOperationsClients(c)
	if err != nil {
		return err
	}
	return startBatchOperation(c, batchClient, frontendClient, &types.BatchOperation{
		OperationType:           types.BatchOperationTypeTerminate.Ptr(),
		TerminateBatchOperation: &types.TerminateBatchOperation{},
	})
}

func StartBatchCancel(c *cli.Context) error {
	batchClient, frontendClient, err := getBatchOperationsClients(c)
	if err != nil {
		return err
	}
	return startBatchOperation(c, batchClient, frontendClient, &types.BatchOperation{
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
	batchClient, frontendClient, err := getBatchOperationsClients(c)
	if err != nil {
		return err
	}
	return startBatchOperation(c, batchClient, frontendClient, &types.BatchOperation{
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
	batchClient, frontendClient, err := getBatchOperationsClients(c)
	if err != nil {
		return err
	}
	return startBatchOperation(c, batchClient, frontendClient, &types.BatchOperation{
		OperationType: types.BatchOperationTypeReplicate.Ptr(),
		ReplicateBatchOperation: &types.ReplicateBatchOperation{
			SourceCluster: sourceCluster,
			TargetCluster: targetCluster,
		},
	})
}

func StartBatchRefresh(c *cli.Context) error {
	batchClient, frontendClient, err := getBatchOperationsClients(c)
	if err != nil {
		return err
	}
	return startBatchOperation(c, batchClient, frontendClient, &types.BatchOperation{
		OperationType:         types.BatchOperationTypeRefresh.Ptr(),
		RefreshBatchOperation: &types.RefreshBatchOperation{},
	})
}
