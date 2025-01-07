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
	"bytes"
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/common/commoncli"
)

func AdminGetGlobalIsolationGroups(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error creating context:", err)
	}
	req := &types.GetGlobalIsolationGroupsRequest{}
	igs, err := adminClient.GetGlobalIsolationGroups(ctx, req)
	if err != nil {
		return commoncli.Problem("failed to get isolation-groups:", err)
	}

	format := c.String(FlagFormat)
	switch format {
	case "json":
		prettyPrintJSONObject(getDeps(c).Output(), igs.IsolationGroups.ToPartitionList())
	default:
		getDeps(c).Output().Write(renderIsolationGroups(igs.IsolationGroups))
	}
	return nil
}

func AdminUpdateGlobalIsolationGroups(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error creating context:", err)
	}
	err = validateIsolationGroupUpdateArgs(
		c.String(FlagDomain),
		c.StringSlice(FlagIsolationGroupSetDrains),
		c.String(FlagJSON),
		c.Bool(FlagIsolationGroupsRemoveAllDrains),
		false,
	)
	if err != nil {
		return commoncli.Problem("invalid args:", err)
	}

	cfg, err := parseIsolationGroupCliInputCfg(
		c.StringSlice(FlagIsolationGroupSetDrains),
		c.String(FlagJSON),
		c.Bool(FlagIsolationGroupsRemoveAllDrains),
	)
	if err != nil {
		return commoncli.Problem("failed to parse input:", err)
	}

	_, err = adminClient.UpdateGlobalIsolationGroups(ctx, &types.UpdateGlobalIsolationGroupsRequest{
		IsolationGroups: *cfg,
	})
	if err != nil {
		return commoncli.Problem("failed to update isolation-groups", fmt.Errorf("used %#v, got %v", cfg, err))
	}
	return nil
}

func AdminGetDomainIsolationGroups(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}
	domain := c.String(FlagDomain)

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error creating context:", err)
	}

	req := &types.GetDomainIsolationGroupsRequest{
		Domain: domain,
	}
	igs, err := adminClient.GetDomainIsolationGroups(ctx, req)
	if err != nil {
		return commoncli.Problem("failed to get isolation-groups:", err)
	}

	format := c.String(FlagFormat)
	switch format {
	case "json":
		prettyPrintJSONObject(getDeps(c).Output(), igs.IsolationGroups.ToPartitionList())
	default:
		getDeps(c).Output().Write([]byte(renderIsolationGroups(igs.IsolationGroups)))
	}
	return nil
}

func AdminUpdateDomainIsolationGroups(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}
	domain := c.String(FlagDomain)

	err = validateIsolationGroupUpdateArgs(
		c.String(FlagDomain),
		c.StringSlice(FlagIsolationGroupSetDrains),
		c.String(FlagJSON),
		c.Bool(FlagIsolationGroupsRemoveAllDrains),
		true,
	)
	if err != nil {
		return commoncli.Problem("invalid args:", err)
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error creating context:", err)
	}

	cfg, err := parseIsolationGroupCliInputCfg(
		c.StringSlice(FlagIsolationGroupSetDrains),
		c.String(FlagJSON),
		c.Bool(FlagIsolationGroupsRemoveAllDrains),
	)
	if err != nil {
		return commoncli.Problem("failed to parse input:", err)
	}

	req := &types.UpdateDomainIsolationGroupsRequest{
		Domain:          domain,
		IsolationGroups: *cfg,
	}
	_, err = adminClient.UpdateDomainIsolationGroups(ctx, req)

	if err != nil {
		return commoncli.Problem("failed to update isolation-groups", fmt.Errorf("used %#v, got %v", req, err))
	}
	return nil
}

func validateIsolationGroupUpdateArgs(
	domainArgs string,
	setDrainsArgs []string,
	jsonCfgArgs string,
	removeAllDrainsArgs bool,
	requiresDomain bool,
) error {
	if requiresDomain {
		if domainArgs == "" {
			return fmt.Errorf("the --domain flag is required")
		}
	}

	if len(setDrainsArgs) == 0 &&
		jsonCfgArgs == "" &&
		!removeAllDrainsArgs {
		return fmt.Errorf("need to specify either %q, %q or %q flags",
			FlagIsolationGroupSetDrains,
			FlagJSON,
			FlagIsolationGroupsRemoveAllDrains,
		)
	}

	if removeAllDrainsArgs && (len(setDrainsArgs) != 0 || jsonCfgArgs != "") {
		return fmt.Errorf("specify either remove or set-drains, not both")
	}

	return nil
}

func parseIsolationGroupCliInputCfg(drains []string, jsonInput string, removeAllDrains bool) (*types.IsolationGroupConfiguration, error) {

	req := types.IsolationGroupConfiguration{}
	if removeAllDrains {
		return &req, nil
	}

	if len(drains) != 0 {
		req := types.IsolationGroupConfiguration{}
		for _, drain := range drains {
			req[drain] = types.IsolationGroupPartition{
				Name:  drain,
				State: types.IsolationGroupStateDrained,
			}
		}
		return &req, nil
	}

	var input []types.IsolationGroupPartition
	err := json.Unmarshal([]byte(jsonInput), &input)

	if err != nil {
		return nil, fmt.Errorf(`failed to marshal input. Trying to marshal []types.IsolationGroupPartition

examples:
- []                                    # will remove all isolation groups
- [{"Name": "zone-123", "State": 2}]    # drain zone-123

%v`, err)
	}
	for _, g := range input {
		req[g.Name] = g
	}

	return &req, nil
}

func renderIsolationGroups(igs types.IsolationGroupConfiguration) []byte {
	output := &bytes.Buffer{}
	w := tabwriter.NewWriter(output, 0, 0, 1, ' ', 0)
	fmt.Fprintln(w, "Isolation Groups\tState")
	if len(igs) == 0 {
		return []byte("-- No groups found --\n")
	}
	for _, v := range igs.ToPartitionList() {
		fmt.Fprintf(w, "%s\t%s\n", v.Name, convertIsolationGroupStateToString(v.State))
	}
	w.Flush()
	return output.Bytes()
}

func convertIsolationGroupStateToString(state types.IsolationGroupState) string {
	switch state {
	case types.IsolationGroupStateDrained:
		return "Drained"
	case types.IsolationGroupStateHealthy:
		return "Healthy"
	default:
		return fmt.Sprintf("Unknown state: %d", state)
	}
}
