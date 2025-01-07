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
	"encoding/json"
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/common/commoncli"
)

func AdminGetAsyncWFConfig(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	domainName, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not present:", err)
	}
	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}

	req := &types.GetDomainAsyncWorkflowConfiguratonRequest{
		Domain: domainName,
	}

	resp, err := adminClient.GetDomainAsyncWorkflowConfiguraton(ctx, req)
	if err != nil {
		return commoncli.Problem("Failed to get async wf queue config", err)
	}

	if resp == nil || resp.Configuration == nil {
		fmt.Printf("Async workflow queue config not found for domain %s\n", domainName)
		return nil
	}

	fmt.Printf("Async workflow queue config for domain %s:\n", domainName)
	prettyPrintJSONObject(getDeps(c).Output(), resp.Configuration)
	return nil
}

func AdminUpdateAsyncWFConfig(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	domainName, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not present:", err)
	}
	asyncWFCfgJSON, err := getRequiredOption(c, FlagJSON)
	if err != nil {
		return commoncli.Problem("Required flag not present:", err)
	}

	var cfg types.AsyncWorkflowConfiguration
	err = json.Unmarshal([]byte(asyncWFCfgJSON), &cfg)
	if err != nil {
		return commoncli.Problem("Failed to parse async workflow config", err)
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}

	req := &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
		Domain:        domainName,
		Configuration: &cfg,
	}

	_, err = adminClient.UpdateDomainAsyncWorkflowConfiguraton(ctx, req)
	if err != nil {
		return commoncli.Problem("Failed to update async workflow queue config", err)
	}

	fmt.Printf("Successfully updated async workflow queue config for domain %s\n", domainName)
	return nil
}
