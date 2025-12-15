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
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/pborman/uuid"
	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/domaindeprecation"
	"github.com/uber/cadence/tools/common/commoncli"
	"github.com/uber/cadence/tools/common/flag"
)

const (
	decisionTimeoutInSeconds    = 5 * 60
	workflowStartToCloseTimeout = 24 * 30 * 60 * 60 // 30 days
)

var (
	gracefulFailoverType = "grace"
)

type (
	domainCLIImpl struct {
		// used when making RPC call to frontend service
		frontendClient      frontend.Client
		frontendAdminClient admin.Client

		// act as admin to modify domain in DB directly
		domainHandler domain.Handler
	}
)

// newDomainCLI creates a domain CLI
func newDomainCLI(
	c *cli.Context,
	isAdminMode bool,
) (*domainCLIImpl, error) {
	d := &domainCLIImpl{}
	var err error
	d.frontendClient, err = initializeFrontendClient(c)
	if err != nil {
		return nil, err
	}
	if isAdminMode {
		d.frontendAdminClient, err = initializeFrontendAdminClient(c)
		if err != nil {
			return nil, err
		}
		d.domainHandler, err = initializeAdminDomainHandler(c)
		if err != nil {
			return nil, err
		}
	}
	return d, nil
}

// RegisterDomain register a domain
func (d *domainCLIImpl) RegisterDomain(c *cli.Context) error {
	domainName, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	description := c.String(FlagDescription)
	ownerEmail := c.String(FlagOwnerEmail)
	retentionDays := defaultDomainRetentionDays

	if c.IsSet(FlagRetentionDays) {
		retentionDays = c.Int(FlagRetentionDays)
	}
	securityToken := c.String(FlagSecurityToken)

	isGlobalDomain := true
	if c.IsSet(FlagIsGlobalDomain) {
		isGlobalDomain, err = strconv.ParseBool(c.String(FlagIsGlobalDomain))
		if err != nil {
			return commoncli.Problem(fmt.Sprintf("Option %s format is invalid.", FlagIsGlobalDomain), err)
		}
	}

	var domainData *flag.StringMap
	if c.IsSet(FlagDomainData) {
		domainData = c.Generic(FlagDomainData).(*flag.StringMap)
	}
	if len(requiredDomainDataKeys) > 0 {
		err = checkRequiredDomainDataKVs(domainData.Value())
		if err != nil {
			return commoncli.Problem("Domain data missed required data.", err)
		}
	}

	activeClusterName := ""
	if c.IsSet(FlagActiveClusterName) {
		activeClusterName = c.String(FlagActiveClusterName)
	}

	var clusters []*types.ClusterReplicationConfiguration
	if c.IsSet(FlagClusters) {
		for _, clusterStr := range c.StringSlice(FlagClusters) {
			clusters = append(clusters, &types.ClusterReplicationConfiguration{
				ClusterName: clusterStr,
			})
		}
	}

	has, err := archivalStatus(c, FlagHistoryArchivalStatus)
	if err != nil {
		return fmt.Errorf("failed to parse %s flag: %w", FlagHistoryArchivalStatus, err)
	}
	vas, err := archivalStatus(c, FlagVisibilityArchivalStatus)
	if err != nil {
		return fmt.Errorf("failed to parse %s flag: %w", FlagVisibilityArchivalStatus, err)
	}

	var activeClusters *types.ActiveClusters
	if c.IsSet(FlagActiveClusters) {
		ac, err := parseActiveClustersByClusterAttribute(c.String(FlagActiveClusters))
		if err != nil {
			return err
		}
		activeClusters = &ac
	}

	request := &types.RegisterDomainRequest{
		Name:                                   domainName,
		Description:                            description,
		OwnerEmail:                             ownerEmail,
		Data:                                   domainData.Value(),
		WorkflowExecutionRetentionPeriodInDays: int32(retentionDays),
		Clusters:                               clusters,
		ActiveClusterName:                      activeClusterName,
		ActiveClusters:                         activeClusters,
		SecurityToken:                          securityToken,
		HistoryArchivalStatus:                  has,
		HistoryArchivalURI:                     c.String(FlagHistoryArchivalURI),
		VisibilityArchivalStatus:               vas,
		VisibilityArchivalURI:                  c.String(FlagVisibilityArchivalURI),
		IsGlobalDomain:                         isGlobalDomain,
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context:", err)
	}
	err = d.registerDomain(ctx, request)
	if err != nil {
		if _, ok := err.(*types.DomainAlreadyExistsError); !ok {
			return commoncli.Problem("Register Domain operation failed.", err)
		}
		return commoncli.Problem(fmt.Sprintf("Domain %s already registered.", domainName), err)
	}
	fmt.Printf("Domain %s successfully registered.\n", domainName)
	return nil
}

// UpdateDomain updates a domain
func (d *domainCLIImpl) UpdateDomain(c *cli.Context) error {
	domainName, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	var updateRequest *types.UpdateDomainRequest
	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	if c.IsSet(FlagActiveClusterName) { // active-passive domain failover
		activeCluster := c.String(FlagActiveClusterName)
		fmt.Printf("Will set active cluster name to: %s, other flag will be omitted.\n", activeCluster)

		var failoverTimeout *int32
		if c.String(FlagFailoverType) == gracefulFailoverType {
			timeout := int32(c.Int(FlagFailoverTimeout))
			failoverTimeout = &timeout
		}

		updateRequest = &types.UpdateDomainRequest{
			Name:                     domainName,
			ActiveClusterName:        common.StringPtr(activeCluster),
			FailoverTimeoutInSeconds: failoverTimeout,
		}
	} else if c.IsSet(FlagActiveClusters) { // active-active domain failover
		activeClusters, err := parseActiveClustersByClusterAttribute(c.String(FlagActiveClusters))
		if err != nil {
			return err
		}

		updateRequest = &types.UpdateDomainRequest{
			Name: domainName,
			ActiveClusters: &types.ActiveClusters{
				AttributeScopes: activeClusters.AttributeScopes,
			},
		}
	} else {
		resp, err := d.describeDomain(ctx, &types.DescribeDomainRequest{
			Name: common.StringPtr(domainName),
		})
		if err != nil {
			if _, ok := err.(*types.EntityNotExistsError); !ok {
				return commoncli.Problem("Operation UpdateDomain failed.", err)
			}
			return commoncli.Problem(fmt.Sprintf("Domain %s does not exist.", domainName), err)
		}

		description := resp.DomainInfo.GetDescription()
		ownerEmail := resp.DomainInfo.GetOwnerEmail()
		retentionDays := resp.Configuration.GetWorkflowExecutionRetentionPeriodInDays()
		emitMetric := resp.Configuration.GetEmitMetric()
		var clusters []*types.ClusterReplicationConfiguration

		if c.IsSet(FlagDescription) {
			description = c.String(FlagDescription)
		}
		if c.IsSet(FlagOwnerEmail) {
			ownerEmail = c.String(FlagOwnerEmail)
		}
		var domainData *flag.StringMap
		if c.IsSet(FlagDomainData) {
			domainData = c.Generic(FlagDomainData).(*flag.StringMap)
		}
		if c.IsSet(FlagRetentionDays) {
			retentionDays = int32(c.Int(FlagRetentionDays))
		}
		if c.IsSet(FlagClusters) {
			for _, clusterStr := range c.StringSlice(FlagClusters) {
				clusters = append(clusters, &types.ClusterReplicationConfiguration{
					ClusterName: clusterStr,
				})
			}
		}

		var binBinaries *types.BadBinaries
		if c.IsSet(FlagAddBadBinary) {
			if !c.IsSet(FlagReason) {
				return commoncli.Problem("Must provide a reason.", nil)
			}
			binChecksum := c.String(FlagAddBadBinary)
			reason := c.String(FlagReason)
			operator := getCurrentUserFromEnv()
			binBinaries = &types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{
					binChecksum: {
						Reason:   reason,
						Operator: operator,
					},
				},
			}
		}

		var badBinaryToDelete *string
		if c.IsSet(FlagRemoveBadBinary) {
			badBinaryToDelete = common.StringPtr(c.String(FlagRemoveBadBinary))
		}

		has, err := archivalStatus(c, FlagHistoryArchivalStatus)
		if err != nil {
			return fmt.Errorf("failed to parse %s flag: %w", FlagHistoryArchivalStatus, err)
		}
		vas, err := archivalStatus(c, FlagVisibilityArchivalStatus)
		if err != nil {
			return fmt.Errorf("failed to parse %s flag: %w", FlagVisibilityArchivalStatus, err)
		}

		updateRequest = &types.UpdateDomainRequest{
			Name:                                   domainName,
			Description:                            common.StringPtr(description),
			OwnerEmail:                             common.StringPtr(ownerEmail),
			Data:                                   domainData.Value(),
			WorkflowExecutionRetentionPeriodInDays: common.Int32Ptr(retentionDays),
			EmitMetric:                             common.BoolPtr(emitMetric),
			HistoryArchivalStatus:                  has,
			HistoryArchivalURI:                     common.StringPtr(c.String(FlagHistoryArchivalURI)),
			VisibilityArchivalStatus:               vas,
			VisibilityArchivalURI:                  common.StringPtr(c.String(FlagVisibilityArchivalURI)),
			BadBinaries:                            binBinaries,
			Clusters:                               clusters,
			DeleteBadBinary:                        badBinaryToDelete,
		}
	}

	securityToken := c.String(FlagSecurityToken)
	updateRequest.SecurityToken = securityToken
	_, err = d.updateDomain(ctx, updateRequest)
	if err != nil {
		var entityNotExistsErr *types.EntityNotExistsError
		if errors.As(err, &entityNotExistsErr) {
			return commoncli.Problem(fmt.Sprintf("Domain %s does not exist.", domainName), err)
		}
		var accessDeniedErr *types.AccessDeniedError
		if errors.As(err, &accessDeniedErr) {
			fmt.Fprintf(os.Stderr, "WARNING: Update domain operation may not be available for general user use and is reserved for administrative operations.\n")
			fmt.Fprintf(os.Stderr, "Please use the 'cadence domain failover' cli-command instead for domain management.\n")
			return commoncli.Problem("Operation UpdateDomain failed due to authorization.", err)
		}
		return commoncli.Problem("Operation UpdateDomain failed.", err)
	}
	fmt.Printf("Domain %s successfully updated.\n", domainName)
	return nil
}

func (d *domainCLIImpl) DeleteDomain(c *cli.Context) error {
	domainName, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	securityToken := c.String(FlagSecurityToken)

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}

	// First describe the domain to show its details
	describeRequest := &types.DescribeDomainRequest{
		Name: &domainName,
	}
	resp, err := d.describeDomain(ctx, describeRequest)
	if err != nil {
		if _, ok := err.(*types.EntityNotExistsError); !ok {
			return commoncli.Problem("Operation DescribeDomain failed.", err)
		}
		return commoncli.Problem(fmt.Sprintf("Domain %s does not exist.", domainName), err)
	}

	if err := Render(c, newDomainRow(resp), RenderOptions{
		DefaultTemplate: templateDomain,
		Color:           true,
		Border:          true,
		PrintDateTime:   true,
	}); err != nil {
		return fmt.Errorf("failed to render domain details: %w", err)
	}

	fmt.Print("Do you want to delete this domain? \n")
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Please confirm[Yes/No]:")
		text, err := reader.ReadString('\n')
		if err != nil {
			return commoncli.Problem("Failed to get confirmation for domain deletion", err)
		}
		if strings.EqualFold(strings.TrimSpace(text), "yes") {
			break
		} else {
			fmt.Println("Domain deletion cancelled")
			return nil
		}
	}

	err = d.deleteDomain(ctx, &types.DeleteDomainRequest{
		Name:          domainName,
		SecurityToken: securityToken,
	})
	if err != nil {
		return commoncli.Problem("Operation Delete domain failed.", err)
	}
	fmt.Printf("Domain %s successfully deleted.\n", domainName)
	return nil
}

func (d *domainCLIImpl) DeprecateDomain(c *cli.Context) error {
	domainName, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not provided: ", err)
	}

	securityToken := c.String(FlagSecurityToken)

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}

	frontendClient, err := getDeps(c).ServerFrontendClient(c)
	if err != nil {
		return err
	}

	params := domaindeprecation.DomainDeprecationParams{
		DomainName:    domainName,
		SecurityToken: securityToken,
	}
	input, err := json.Marshal(params)
	if err != nil {
		return commoncli.Problem("Failed to encode domain deprecation parameters", err)
	}

	workflowID := uuid.NewRandom().String()
	startRequest := &types.StartWorkflowExecutionRequest{
		Domain:     constants.SystemLocalDomainName,
		WorkflowID: fmt.Sprintf("domain-deprecation-%s-%s", domainName, workflowID),
		WorkflowType: &types.WorkflowType{
			Name: domaindeprecation.DomainDeprecationWorkflowTypeName,
		},
		TaskList: &types.TaskList{
			Name: domaindeprecation.DomainDeprecationTaskListName,
		},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(int32(workflowStartToCloseTimeout)),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(decisionTimeoutInSeconds),
		RequestID:                           uuid.New(),
		Input:                               input,
	}

	resp, err := frontendClient.StartWorkflowExecution(ctx, startRequest)
	if err != nil {
		return commoncli.Problem("Failed to start domain deprecation workflow", err)
	}

	fmt.Printf("Domain deprecation is in progress. Workflow ID: %s, Run ID: %s\n", startRequest.WorkflowID, resp.GetRunID())
	return nil
}

// FailoverDomain fails over a single domain to a target cluster
func (d *domainCLIImpl) FailoverDomain(c *cli.Context) error {
	domainName, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}

	failoverRequest := &types.FailoverDomainRequest{
		DomainName: domainName,
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}

	if c.IsSet(FlagActiveClustersJSON) && c.IsSet(FlagActiveClusters) {
		return commoncli.Problem("Cannot use both --active_clusters_json and --active_clusters flags.", nil)
	}

	if !c.IsSet(FlagActiveClusterName) && !c.IsSet(FlagActiveClusters) && !c.IsSet(FlagActiveClustersJSON) {
		return commoncli.Problem("At least one of the flags --active_cluster, --active_clusters or --active_clusters_json must be provided.", nil)
	}

	if c.IsSet(FlagActiveClusterName) { // active-passive domain failover
		activeCluster := c.String(FlagActiveClusterName)
		failoverRequest.DomainActiveClusterName = common.StringPtr(activeCluster)
	}
	if c.IsSet(FlagActiveClusters) { // active-active domain failover
		ac, err := parseActiveClustersByClusterAttribute(c.String(FlagActiveClusters))
		if err != nil {
			return err
		}
		failoverRequest.ActiveClusters = &ac
	}

	if c.IsSet(FlagActiveClustersJSON) {
		ac, err := parseActiveClustersByClusterAttributeFromJSON(c.String(FlagActiveClustersJSON))
		if err != nil {
			return err
		}
		failoverRequest.ActiveClusters = &ac
	}
	_, err = d.failoverDomain(ctx, failoverRequest)
	if err != nil {
		if _, ok := err.(*types.EntityNotExistsError); ok {
			return commoncli.Problem(fmt.Sprintf("Domain %s does not exist.", domainName), err)
		}
		return commoncli.Problem("Operation FailoverDomain failed.", err)
	}
	fmt.Printf("Domain %s successfully failed over.\n", domainName)
	return nil
}

// FailoverDomains is used for managed failover all domains with domain data IsManagedByCadence=true
func (d *domainCLIImpl) FailoverDomains(c *cli.Context) error {
	// ask user for confirmation
	prompt("You are trying to failover all managed domains, continue? y/N")
	_, _, err := d.failoverDomains(c)
	return err
}

// return succeed and failed domains for testing purpose
func (d *domainCLIImpl) failoverDomains(c *cli.Context) ([]string, []string, error) {
	targetCluster, err := getRequiredOption(c, FlagActiveClusterName)
	if err != nil {
		return nil, nil, commoncli.Problem("Required flag not found: ", err)
	}
	domains, err := d.getAllDomains(c)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to list domains: %w", err)
	}
	shouldFailover := func(domain *types.DescribeDomainResponse) bool {
		isDomainNotActiveInTargetCluster := domain.ReplicationConfiguration.GetActiveClusterName() != targetCluster
		return isDomainNotActiveInTargetCluster && isDomainFailoverManagedByCadence(domain)
	}
	var succeedDomains []string
	var failedDomains []string
	for _, domain := range domains {
		if shouldFailover(domain) {
			domainName := domain.GetDomainInfo().GetName()
			err := d.failover(c, domainName, targetCluster)
			if err != nil {
				printError(getDeps(c).Output(), fmt.Sprintf("Failed failover domain: %s\n", domainName), err)
				failedDomains = append(failedDomains, domainName)
			} else {
				fmt.Printf("Success failover domain: %s\n", domainName)
				succeedDomains = append(succeedDomains, domainName)
			}
		}
	}
	fmt.Printf("Succeed %d: %v\n", len(succeedDomains), succeedDomains)
	fmt.Printf("Failed  %d: %v\n", len(failedDomains), failedDomains)
	return succeedDomains, failedDomains, nil
}

func (d *domainCLIImpl) getAllDomains(c *cli.Context) ([]*types.DescribeDomainResponse, error) {
	var res []*types.DescribeDomainResponse
	pagesize := int32(200)
	var token []byte
	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return nil, commoncli.Problem("Error in creating context: ", err)
	}
	for more := true; more; more = len(token) > 0 {
		listRequest := &types.ListDomainsRequest{
			PageSize:      pagesize,
			NextPageToken: token,
		}
		listResp, err := d.listDomains(ctx, listRequest)
		if err != nil {
			return nil, commoncli.Problem("Error when list domains info", err)
		}
		token = listResp.GetNextPageToken()
		res = append(res, listResp.GetDomains()...)
	}
	return res, nil
}

func isDomainFailoverManagedByCadence(domain *types.DescribeDomainResponse) bool {
	domainData := domain.DomainInfo.GetData()
	return strings.ToLower(strings.TrimSpace(domainData[constants.DomainDataKeyForManagedFailover])) == "true"
}

func (d *domainCLIImpl) failover(c *cli.Context, domainName string, targetCluster string) error {
	updateRequest := &types.UpdateDomainRequest{
		Name:              domainName,
		ActiveClusterName: common.StringPtr(targetCluster),
	}
	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	_, err = d.updateDomain(ctx, updateRequest)
	return err
}

var templateDomain = `Name: {{.Name}}
UUID: {{.UUID}}
Description: {{.Description}}
OwnerEmail: {{.OwnerEmail}}
DomainData: {{.DomainData}}
Status: {{.Status}}
RetentionInDays: {{.RetentionDays}}
EmitMetrics: {{.EmitMetrics}}
IsGlobal(XDC)Domain: {{.IsGlobal}}
ActiveClusterName: {{.ActiveCluster}}
IsActiveActiveDomain: {{.IsActiveActiveDomain}}
Clusters: {{if .IsGlobal}}{{.Clusters}}{{else}}N/A, Not a global domain{{end}}
HistoryArchivalStatus: {{.HistoryArchivalStatus}}{{with .HistoryArchivalURI}}
HistoryArchivalURI: {{.}}{{end}}
VisibilityArchivalStatus: {{.VisibilityArchivalStatus}}{{with .VisibilityArchivalURI}}
VisibilityArchivalURI: {{.}}{{end}}
{{with .BadBinaries}}Bad binaries to reset:
{{table .}}{{end}}
{{with .FailoverInfo}}Graceful failover info:
{{table .}}{{end}}
{{with .ActiveClustersByClusterAttribute}}Active clusters by cluster attribute:
{{table .}}{{end}}`

// DescribeDomain updates a domain
func (d *domainCLIImpl) DescribeDomain(c *cli.Context) error {
	domainName := c.String(FlagDomain)
	domainID := c.String(FlagDomainID)
	printJSON := c.Bool(FlagPrintJSON)

	request := types.DescribeDomainRequest{}
	if domainID != "" {
		request.UUID = &domainID
	}
	if domainName != "" {
		request.Name = &domainName
	}
	if domainID == "" && domainName == "" {
		return commoncli.Problem("At least domainID or domainName must be provided.", nil)
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	resp, err := d.describeDomain(ctx, &request)
	if err != nil {
		if _, ok := err.(*types.EntityNotExistsError); !ok {
			return commoncli.Problem("Operation DescribeDomain failed.", err)
		}
		return commoncli.Problem(fmt.Sprintf("Domain %s does not exist.", domainName), err)
	}

	if printJSON {
		output, err := json.Marshal(resp)
		if err != nil {
			return commoncli.Problem("Failed to encode domain response into JSON.", err)
		}
		fmt.Println(string(output))
		return nil
	}

	return Render(c, newDomainRow(resp), RenderOptions{
		DefaultTemplate: templateDomain,
		Color:           true,
		Border:          true,
		PrintDateTime:   true,
	})
}

type BadBinaryRow struct {
	Checksum  string    `header:"Binary Checksum"`
	Operator  string    `header:"Operator"`
	StartTime time.Time `header:"Start Time"`
	Reason    string    `header:"Reason"`
}

type FailoverInfoRow struct {
	FailoverVersion     int64     `header:"Failover Version"`
	StartTime           time.Time `header:"Start Time"`
	ExpireTime          time.Time `header:"Expire Time"`
	CompletedShardCount int32     `header:"Completed Shard Count"`
	PendingShard        []int32   `header:"Pending Shard"`
}

type ActiveClusterInfoRow struct {
	Scope             string `header:"Scope"`
	Value             string `header:"Value"`
	ActiveClusterName string `header:"Active Cluster Name"`
	FailoverVersion   int64  `header:"Failover Version"`
}

type FailoverHistoryRow struct {
	EventID     string    `header:"Event ID"`
	CreatedTime time.Time `header:"Created Time"`
	FromCluster string    `header:"From Cluster"`
	ToCluster   string    `header:"To Cluster"`
	Attribute   string    `header:"Cluster Attribute"`
}

type DomainRow struct {
	Name                             string `header:"Name"`
	UUID                             string `header:"UUID"`
	Description                      string
	OwnerEmail                       string
	DomainData                       map[string]string  `header:"Domain Data"`
	Status                           types.DomainStatus `header:"Status"`
	IsGlobal                         bool               `header:"Is Global Domain"`
	ActiveCluster                    string             `header:"Active Cluster"`
	Clusters                         []string           `header:"Clusters"`
	RetentionDays                    int32              `header:"Retention Days"`
	EmitMetrics                      bool
	HistoryArchivalStatus            types.ArchivalStatus `header:"History Archival Status"`
	HistoryArchivalURI               string               `header:"History Archival URI"`
	VisibilityArchivalStatus         types.ArchivalStatus `header:"Visibility Archival Status"`
	VisibilityArchivalURI            string               `header:"Visibility Archival URI"`
	BadBinaries                      []BadBinaryRow
	FailoverInfo                     *FailoverInfoRow
	LongRunningWorkFlowNum           *int
	IsActiveActiveDomain             bool
	ActiveClustersByClusterAttribute []ActiveClusterInfoRow
}

type DomainMigrationRow struct {
	ValidationCheck   string `header:"Validation Checker"`
	ValidationResult  bool   `header:"Validation Result"`
	ValidationDetails ValidationDetails
}

type ValidationDetails struct {
	CurrentDomainRow            *types.DescribeDomainResponse
	NewDomainRow                *types.DescribeDomainResponse
	MismatchedDomainMetaData    string
	LongRunningWorkFlowNum      *int
	MismatchedDynamicConfig     []MismatchedDynamicConfig
	MissingCurrSearchAttributes []string
	MissingNewSearchAttributes  []string
}

type MismatchedDynamicConfig struct {
	Key        dynamicproperties.Key
	CurrValues []*types.DynamicConfigValue
	NewValues  []*types.DynamicConfigValue
}

func newDomainRow(domain *types.DescribeDomainResponse) DomainRow {
	return DomainRow{
		Name:                             domain.DomainInfo.Name,
		UUID:                             domain.DomainInfo.UUID,
		Description:                      domain.DomainInfo.Description,
		OwnerEmail:                       domain.DomainInfo.OwnerEmail,
		DomainData:                       domain.DomainInfo.GetData(),
		Status:                           domain.DomainInfo.GetStatus(),
		IsGlobal:                         domain.IsGlobalDomain,
		ActiveCluster:                    domain.ReplicationConfiguration.GetActiveClusterName(),
		Clusters:                         clustersToStrings(domain.ReplicationConfiguration.GetClusters()),
		RetentionDays:                    domain.Configuration.GetWorkflowExecutionRetentionPeriodInDays(),
		EmitMetrics:                      domain.Configuration.GetEmitMetric(),
		HistoryArchivalStatus:            domain.Configuration.GetHistoryArchivalStatus(),
		HistoryArchivalURI:               domain.Configuration.GetHistoryArchivalURI(),
		VisibilityArchivalStatus:         domain.Configuration.GetVisibilityArchivalStatus(),
		VisibilityArchivalURI:            domain.Configuration.GetVisibilityArchivalURI(),
		BadBinaries:                      newBadBinaryRows(domain.Configuration.BadBinaries),
		FailoverInfo:                     newFailoverInfoRow(domain.FailoverInfo),
		IsActiveActiveDomain:             domain.ReplicationConfiguration.IsActiveActive(),
		ActiveClustersByClusterAttribute: newActiveClustersByClusterAttribute(domain.ReplicationConfiguration.GetActiveClusters()),
	}
}

func newFailoverInfoRow(info *types.FailoverInfo) *FailoverInfoRow {
	if info == nil {
		return nil
	}
	return &FailoverInfoRow{
		FailoverVersion:     info.GetFailoverVersion(),
		StartTime:           time.Unix(0, info.GetFailoverStartTimestamp()),
		ExpireTime:          time.Unix(0, info.GetFailoverExpireTimestamp()),
		CompletedShardCount: info.GetCompletedShardCount(),
		PendingShard:        info.GetPendingShards(),
	}
}

func newBadBinaryRows(bb *types.BadBinaries) []BadBinaryRow {
	if bb == nil {
		return nil
	}
	rows := []BadBinaryRow{}
	for cs, bin := range bb.Binaries {
		rows = append(rows, BadBinaryRow{
			Checksum:  cs,
			Operator:  bin.GetOperator(),
			StartTime: time.Unix(0, bin.GetCreatedTimeNano()),
			Reason:    bin.GetReason(),
		})
	}
	return rows
}

func newActiveClustersByClusterAttribute(activeClusters *types.ActiveClusters) []ActiveClusterInfoRow {
	if activeClusters == nil {
		return nil
	}

	rows := []ActiveClusterInfoRow{}
	for scope, attributeScopes := range activeClusters.GetAttributeScopes() {
		for attributeScope, clusterInfo := range attributeScopes.ClusterAttributes {
			rows = append(rows, ActiveClusterInfoRow{
				Scope:             scope,
				Value:             attributeScope,
				ActiveClusterName: clusterInfo.ActiveClusterName,
				FailoverVersion:   clusterInfo.FailoverVersion,
			})
		}
	}

	// Sort for deterministic output
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Scope != rows[j].Scope {
			return rows[i].Scope < rows[j].Scope
		}
		return rows[i].Value < rows[j].Value
	})

	return rows
}

func renderFailoverHistoryTable(response *types.ListFailoverHistoryResponse) {
	renderFailoverHistoryTableToWriter(os.Stdout, response)
}

func renderFailoverHistoryTableToWriter(writer interface{ Write([]byte) (int, error) }, response *types.ListFailoverHistoryResponse) {
	table := tablewriter.NewWriter(writer)
	table.SetRowLine(true)
	table.SetBorder(true)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetAutoMergeCells(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	displayClusterAttributeCol := false
	var columnsToRender [][]string

	for _, event := range response.GetFailoverEvents() {
		eventID := event.GetID()
		createdTime := time.Unix(0, event.GetCreatedTime()).Format(time.RFC3339)
		clusterFailovers := event.GetClusterFailovers()

		if len(clusterFailovers) == 0 {
			// If no cluster failovers, show event info only
			table.Append([]string{eventID, createdTime, "-", "-", "-"})
			continue
		}

		for _, failover := range clusterFailovers {
			fromCluster := ""
			if fromInfo := failover.GetFromCluster(); fromInfo != nil {
				fromCluster = fromInfo.ActiveClusterName
			}
			toCluster := ""
			if toInfo := failover.GetToCluster(); toInfo != nil {
				toCluster = toInfo.ActiveClusterName
			}
			domainFailover := fmt.Sprintf("%s -> %s", fromCluster, toCluster)

			attribute := ""
			if attr := failover.GetClusterAttribute(); attr != nil {
				if attr.Scope != "" && attr.Name != "" {
					attribute = fmt.Sprintf("%s.%s", attr.Scope, attr.Name)
					displayClusterAttributeCol = true
				}
			}
			if displayClusterAttributeCol {
				columnsToRender = append(columnsToRender, []string{eventID, createdTime, domainFailover, attribute})
			} else {
				columnsToRender = append(columnsToRender, []string{eventID, createdTime, domainFailover})
			}
		}
	}
	table.AppendBulk(columnsToRender)

	if displayClusterAttributeCol {
		table.SetHeader([]string{"Event ID", "Failover Timestamp", "Failover", "Cluster Attribute"})
	} else {
		table.SetHeader([]string{"Event ID", "Failover Timestamp", "Failover"})
	}

	table.Render()
}

func domainTableOptions(c *cli.Context) RenderOptions {
	printAll := c.Bool(FlagAll)
	printFull := c.Bool(FlagPrintFullyDetail)

	return RenderOptions{
		DefaultTemplate: templateTable,
		Color:           true,
		OptionalColumns: map[string]bool{
			"Status":                     printAll || printFull,
			"Clusters":                   printFull,
			"Retention Days":             printFull,
			"History Archival Status":    printFull,
			"History Archival URI":       printFull,
			"Visibility Archival Status": printFull,
			"Visibility Archival URI":    printFull,
		},
	}
}

func (d *domainCLIImpl) ListDomains(c *cli.Context) error {
	output := getDeps(c).Output()

	pageSize := c.Int(FlagPageSize)
	prefix := c.String(FlagPrefix)
	printAll := c.Bool(FlagAll)
	printDeprecated := c.Bool(FlagDeprecated)
	printJSON := c.Bool(FlagPrintJSON)

	if printAll && printDeprecated {
		return commoncli.Problem(fmt.Sprintf("Cannot specify %s and %s flags at the same time.", FlagAll, FlagDeprecated), nil)
	}

	domains, err := d.getAllDomains(c)
	if err != nil {
		return err
	}
	var filteredDomains []*types.DescribeDomainResponse

	// Only list domains that are matching to the prefix if prefix is provided
	if len(prefix) > 0 {
		var prefixDomains []*types.DescribeDomainResponse
		for _, domain := range domains {
			if strings.Index(domain.DomainInfo.Name, prefix) == 0 {
				prefixDomains = append(prefixDomains, domain)
			}
		}
		domains = prefixDomains
	}

	if printAll {
		filteredDomains = domains
	} else {
		filteredDomains = make([]*types.DescribeDomainResponse, 0, len(domains))
		for _, domain := range domains {
			if printDeprecated && *domain.DomainInfo.Status == types.DomainStatusDeprecated {
				filteredDomains = append(filteredDomains, domain)
			} else if !printDeprecated && *domain.DomainInfo.Status == types.DomainStatusRegistered {
				filteredDomains = append(filteredDomains, domain)
			}
		}
	}

	if printJSON {
		output, err := json.Marshal(filteredDomains)
		if err != nil {
			return commoncli.Problem("Failed to encode domain results into JSON.", err)
		}
		fmt.Println(string(output))
		return nil
	}

	table := make([]DomainRow, 0, pageSize)

	currentPageSize := 0
	for i, domain := range filteredDomains {
		table = append(table, newDomainRow(domain))
		currentPageSize++

		if currentPageSize != pageSize {
			continue
		}

		// page is full
		if err := Render(c, table, domainTableOptions(c)); err != nil {
			return fmt.Errorf("failed to render domain list: %w", err)
		}
		if i == len(domains)-1 || !showNextPage(output) {
			return nil
		}
		table = make([]DomainRow, 0, pageSize)
		currentPageSize = 0
	}

	return Render(c, table, domainTableOptions(c))
}

func (d *domainCLIImpl) listDomains(
	ctx context.Context,
	request *types.ListDomainsRequest,
) (*types.ListDomainsResponse, error) {

	if d.frontendClient != nil {
		return d.frontendClient.ListDomains(ctx, request)
	}

	return d.domainHandler.ListDomains(ctx, request)
}

func (d *domainCLIImpl) registerDomain(
	ctx context.Context,
	request *types.RegisterDomainRequest,
) error {

	if d.frontendClient != nil {
		return d.frontendClient.RegisterDomain(ctx, request)
	}

	return d.domainHandler.RegisterDomain(ctx, request)
}

func (d *domainCLIImpl) updateDomain(
	ctx context.Context,
	request *types.UpdateDomainRequest,
) (*types.UpdateDomainResponse, error) {

	if d.frontendClient != nil {
		return d.frontendClient.UpdateDomain(ctx, request)
	}

	return d.domainHandler.UpdateDomain(ctx, request)
}

func (d *domainCLIImpl) deleteDomain(
	ctx context.Context,
	request *types.DeleteDomainRequest,
) error {

	if d.frontendClient != nil {
		return d.frontendClient.DeleteDomain(ctx, request)
	}

	return d.domainHandler.DeleteDomain(ctx, request)
}

// ListFailoverHistory lists the failover history for a domain
func (d *domainCLIImpl) ListFailoverHistory(c *cli.Context) error {
	domainName, err := getRequiredOption(c, FlagDomain)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}

	// Get domain ID by describing the domain first
	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}

	describeResp, err := d.describeDomain(ctx, &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	if err != nil {
		if _, ok := err.(*types.EntityNotExistsError); ok {
			return commoncli.Problem(fmt.Sprintf("Domain %s does not exist.", domainName), err)
		}
		return commoncli.Problem("Failed to describe domain.", err)
	}

	domainID := describeResp.DomainInfo.GetUUID()

	limit := 10
	if c.Bool(FlagAll) {
		limit = -1
	}

	printJSON := c.Bool(FlagPrintJSON)
	allResponses := &types.ListFailoverHistoryResponse{}

	var nextPageToken []byte

	for {
		request := &types.ListFailoverHistoryRequest{
			Filters: &types.ListFailoverHistoryRequestFilters{
				DomainID: domainID,
			},
			Pagination: &types.PaginationOptions{
				PageSize:      common.Int32Ptr(int32(10)),
				NextPageToken: nextPageToken,
			},
		}

		ctx, cancel, err = newContext(c)
		if err != nil {
			return commoncli.Problem("Error in creating context: ", err)
		}

		resp, err := d.frontendClient.ListFailoverHistory(ctx, request)
		cancel()
		if err != nil {
			return commoncli.Problem("Failed to list failover history.", err)
		}
		allResponses.FailoverEvents = append(allResponses.FailoverEvents, resp.FailoverEvents...)
		nextPageToken = resp.NextPageToken
		if len(nextPageToken) == 0 || nextPageToken == nil {
			break
		}
		if limit > 0 && len(allResponses.FailoverEvents) >= int(limit) {
			break
		}
	}

	if printJSON {
		output, err := json.Marshal(allResponses)
		if err != nil {
			return commoncli.Problem("Failed to encode failover history into JSON.", err)
		}
		fmt.Println(string(output))
		return nil
	}

	if len(allResponses.GetFailoverEvents()) == 0 {
		fmt.Println("No failover history found for domain:", domainName)
		return nil
	}

	renderFailoverHistoryTable(allResponses)
	return nil
}

func (d *domainCLIImpl) describeDomain(
	ctx context.Context,
	request *types.DescribeDomainRequest,
) (*types.DescribeDomainResponse, error) {

	if d.frontendClient != nil {
		return d.frontendClient.DescribeDomain(ctx, request)
	}

	return d.domainHandler.DescribeDomain(ctx, request)
}

func (d *domainCLIImpl) failoverDomain(
	ctx context.Context,
	request *types.FailoverDomainRequest,
) (*types.FailoverDomainResponse, error) {

	if d.frontendClient != nil {
		return d.frontendClient.FailoverDomain(ctx, request)
	}

	return d.domainHandler.FailoverDomain(ctx, request)
}

func archivalStatus(c *cli.Context, statusFlagName string) (*types.ArchivalStatus, error) {
	if c.IsSet(statusFlagName) {
		switch c.String(statusFlagName) {
		case "disabled":
			return types.ArchivalStatusDisabled.Ptr(), nil
		case "enabled":
			return types.ArchivalStatusEnabled.Ptr(), nil
		default:
			return nil, commoncli.Problem(fmt.Sprintf("Option %s format is invalid.", statusFlagName), errors.New("invalid status, valid values are \"disabled\" and \"enabled\""))
		}
	}
	return nil, nil
}

func clustersToStrings(clusters []*types.ClusterReplicationConfiguration) []string {
	var res []string
	for _, cluster := range clusters {
		res = append(res, cluster.GetClusterName())
	}
	return res
}

func parseActiveClustersByClusterAttributeFromJSON(jsonStr string) (types.ActiveClusters, error) {
	ac := types.ActiveClusters{}
	if err := json.Unmarshal([]byte(jsonStr), &ac); err != nil {
		return types.ActiveClusters{}, fmt.Errorf("couldn't parse the JSON: %w. Got %s", err, jsonStr)
	}
	return ac, nil
}

func parseActiveClustersByClusterAttribute(clusters string) (types.ActiveClusters, error) {
	split := regexp.MustCompile(`(?P<attribute>[a-zA-Z0-9_-]+).(?P<scope>[a-zA-Z0-9_-]+):(?P<name>[a-zA-Z0-9_-]+)`)
	matches := split.FindAllStringSubmatch(clusters, -1)
	if len(matches) == 0 {
		return types.ActiveClusters{}, fmt.Errorf("option %s format is invalid. Expected format is 'region.dca:dev2_dca,region.phx:dev2_phx'", FlagActiveClusters)
	}

	out := types.ActiveClusters{
		AttributeScopes: map[string]types.ClusterAttributeScope{},
	}

	for _, match := range matches {
		if len(match) != 4 {
			return types.ActiveClusters{}, fmt.Errorf("option %s format is invalid. Expected format is 'region.dca:dev2_dca,region.phx:dev2_phx'", FlagActiveClusters)
		}
		attribute := match[1]
		scope := match[2]
		name := match[3]

		existing, ok := out.AttributeScopes[attribute]
		if !ok {
			out.AttributeScopes[attribute] = types.ClusterAttributeScope{
				ClusterAttributes: map[string]types.ActiveClusterInfo{
					scope: {ActiveClusterName: name},
				},
			}
		} else {
			if _, ok := existing.ClusterAttributes[scope]; ok {
				return types.ActiveClusters{}, fmt.Errorf("option active_clusters format is invalid. the key %q was duplicated. This can only map to a single active cluster", scope)
			}
			existing.ClusterAttributes[scope] = types.ActiveClusterInfo{ActiveClusterName: name}
			out.AttributeScopes[attribute] = existing
		}

		out.AttributeScopes[attribute].ClusterAttributes[scope] = types.ActiveClusterInfo{ActiveClusterName: name}
	}

	return out, nil
}
