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
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/pborman/uuid"
	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/visibility"
	"github.com/uber/cadence/service/worker/failovermanager"
	"github.com/uber/cadence/tools/common/commoncli"
)

// An indirection for the prompt function so that it can be mocked in the unit tests
var promptFn = prompt

// AdminAddSearchAttribute to whitelist search attribute
func AdminAddSearchAttribute(c *cli.Context) error {
	key, err := getRequiredOption(c, FlagSearchAttributesKey)
	if err != nil {
		return commoncli.Problem("Required flag not present:", err)
	}
	if err := visibility.ValidateSearchAttributeKey(key); err != nil {
		return commoncli.Problem("Invalid search-attribute key.", err)
	}

	valType, err := getRequiredIntOption(c, FlagSearchAttributesType)
	if err != nil {
		return commoncli.Problem("Required flag not present:", err)
	}
	if !isValueTypeValid(valType) {
		return commoncli.Problem("Unknown Search Attributes value type.", nil)
	}

	// ask user for confirmation
	promptMsg := fmt.Sprintf("Are you trying to add key [%s] with Type [%s]? y/N",
		color.YellowString(key), color.YellowString(intValTypeToString(valType)))
	promptFn(promptMsg)

	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}
	ctx, cancel, err := newContext(c)
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	defer cancel()
	request := &types.AddSearchAttributeRequest{
		SearchAttribute: map[string]types.IndexedValueType{
			key: types.IndexedValueType(valType),
		},
		SecurityToken: c.String(FlagSecurityToken),
	}

	err = adminClient.AddSearchAttribute(ctx, request)
	if err != nil {
		return commoncli.Problem("Add search attribute failed.", err)
	}
	fmt.Println("Success. Note that for a multil-node Cadence cluster, DynamicConfig MUST be updated separately to whitelist the new attributes.")
	return nil
}

// AdminDescribeCluster is used to dump information about the cluster
func AdminDescribeCluster(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	response, err := adminClient.DescribeCluster(ctx)
	if err != nil {
		return commoncli.Problem("Operation DescribeCluster failed.", err)
	}

	prettyPrintJSONObject(getDeps(c).Output(), response)
	return nil
}

func AdminRebalanceStart(c *cli.Context) error {
	client, err := getCadenceClient(c)
	if err != nil {
		return err
	}
	tcCtx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Context not created:", err)
	}
	workflowID := failovermanager.RebalanceWorkflowID
	rbParams := &failovermanager.RebalanceParams{
		BatchFailoverSize:              100,
		BatchFailoverWaitTimeInSeconds: 10,
	}
	input, err := json.Marshal(rbParams)
	if err != nil {
		return commoncli.Problem("Failed to serialize params for failover workflow", err)
	}
	op, err := getOperator()
	if err != nil {
		return commoncli.Problem("Failed to get operator", err)
	}
	memo, err := getWorkflowMemo(map[string]interface{}{
		common.MemoKeyForOperator: op,
	})
	if err != nil {
		return commoncli.Problem("Failed to serialize memo", err)
	}
	request := &types.StartWorkflowExecutionRequest{
		Domain:                              common.SystemLocalDomainName,
		WorkflowID:                          workflowID,
		RequestID:                           uuid.New(),
		Identity:                            getCliIdentity(),
		WorkflowIDReusePolicy:               types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(int32(defaultDecisionTimeoutInSeconds)),
		Input:                               input,
		TaskList: &types.TaskList{
			Name: failovermanager.TaskListName,
		},
		Memo: memo,
		WorkflowType: &types.WorkflowType{
			Name: failovermanager.RebalanceWorkflowTypeName,
		},
	}

	resp, err := client.StartWorkflowExecution(tcCtx, request)
	if err != nil {
		return commoncli.Problem("Failed to start failover workflow", err)
	}

	output := getDeps(c).Output()
	output.Write([]byte("Rebalance workflow started\n"))
	output.Write([]byte("wid: " + workflowID + "\n"))
	output.Write([]byte("rid: " + resp.GetRunID() + "\n"))

	return nil
}

func AdminRebalanceList(c *cli.Context) error {
	if err := c.Set(FlagWorkflowID, failovermanager.RebalanceWorkflowID); err != nil {
		return err
	}
	if err := c.Set(FlagDomain, common.SystemLocalDomainName); err != nil {
		return err
	}
	return ListWorkflow(c)
}

func intValTypeToString(valType int) string {
	switch valType {
	case 0:
		return "String"
	case 1:
		return "Keyword"
	case 2:
		return "Int"
	case 3:
		return "Double"
	case 4:
		return "Bool"
	case 5:
		return "Datetime"
	default:
		return ""
	}
}

func isValueTypeValid(valType int) bool {
	return valType >= 0 && valType <= 5
}
