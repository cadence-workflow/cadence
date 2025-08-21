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
	"strconv"
	"strings"
	"time"

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
	isActiveActiveDomain := false
	if c.IsSet(FlagIsActiveActiveDomain) {
		isActiveActiveDomain, err = strconv.ParseBool(c.String(FlagIsActiveActiveDomain))
		if err != nil {
			return commoncli.Problem(fmt.Sprintf("Option %s format is invalid.", FlagIsActiveActiveDomain), err)
		}

		// Also set isGlobalDomain to true if it is active-active domain
		isGlobalDomain = isActiveActiveDomain
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
		if isActiveActiveDomain {
			return commoncli.Problem("Option --active_cluster is not supported for active-active domain. Use --active_clusters_by_region instead.", nil)
		}
		activeClusterName = c.String(FlagActiveClusterName)
	}

	activeClustersByRegion, err := parseActiveClustersByRegion(c, isActiveActiveDomain)
	if err != nil {
		return err
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

	request := &types.RegisterDomainRequest{
		Name:                                   domainName,
		Description:                            description,
		OwnerEmail:                             ownerEmail,
		Data:                                   domainData.Value(),
		WorkflowExecutionRetentionPeriodInDays: int32(retentionDays),
		Clusters:                               clusters,
		ActiveClusterName:                      activeClusterName,
		ActiveClustersByRegion:                 activeClustersByRegion,
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
	} else if c.IsSet(FlagActiveClustersByRegion) { // active-active domain failover
		activeClustersByRegion, err := parseActiveClustersByRegion(c, true)
		if err != nil {
			return err
		}

		acbr := make(map[string]types.ActiveClusterInfo)
		for region, cluster := range activeClustersByRegion {
			acbr[region] = types.ActiveClusterInfo{
				ActiveClusterName: cluster,
			}
		}

		updateRequest = &types.UpdateDomainRequest{
			Name: domainName,
			ActiveClusters: &types.ActiveClusters{
				ActiveClustersByRegion: acbr,
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
		if _, ok := err.(*types.EntityNotExistsError); ok {
			return commoncli.Problem(fmt.Sprintf("Domain %s does not exist.", domainName), err)
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
ActiveClustersByRegion: {{.ActiveClustersByRegion}}
Clusters: {{if .IsGlobal}}{{.Clusters}}{{else}}N/A, Not a global domain{{end}}
HistoryArchivalStatus: {{.HistoryArchivalStatus}}{{with .HistoryArchivalURI}}
HistoryArchivalURI: {{.}}{{end}}
VisibilityArchivalStatus: {{.VisibilityArchivalStatus}}{{with .VisibilityArchivalURI}}
VisibilityArchivalURI: {{.}}{{end}}
{{with .BadBinaries}}Bad binaries to reset:
{{table .}}{{end}}
{{with .FailoverInfo}}Graceful failover info:
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
	Region          string `header:"Region"`
	ClusterName     string `header:"Cluster Name"`
	FailoverVersion int64  `header:"Failover Version"`
}

type DomainRow struct {
	Name                     string `header:"Name"`
	UUID                     string `header:"UUID"`
	Description              string
	OwnerEmail               string
	DomainData               map[string]string  `header:"Domain Data"`
	Status                   types.DomainStatus `header:"Status"`
	IsGlobal                 bool               `header:"Is Global Domain"`
	ActiveCluster            string             `header:"Active Cluster"`
	Clusters                 []string           `header:"Clusters"`
	RetentionDays            int32              `header:"Retention Days"`
	EmitMetrics              bool
	HistoryArchivalStatus    types.ArchivalStatus `header:"History Archival Status"`
	HistoryArchivalURI       string               `header:"History Archival URI"`
	VisibilityArchivalStatus types.ArchivalStatus `header:"Visibility Archival Status"`
	VisibilityArchivalURI    string               `header:"Visibility Archival URI"`
	BadBinaries              []BadBinaryRow
	FailoverInfo             *FailoverInfoRow
	LongRunningWorkFlowNum   *int
	IsActiveActiveDomain     bool
	ActiveClustersByRegion   []ActiveClusterInfoRow
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
		Name:                     domain.DomainInfo.Name,
		UUID:                     domain.DomainInfo.UUID,
		Description:              domain.DomainInfo.Description,
		OwnerEmail:               domain.DomainInfo.OwnerEmail,
		DomainData:               domain.DomainInfo.GetData(),
		Status:                   domain.DomainInfo.GetStatus(),
		IsGlobal:                 domain.IsGlobalDomain,
		ActiveCluster:            domain.ReplicationConfiguration.GetActiveClusterName(),
		Clusters:                 clustersToStrings(domain.ReplicationConfiguration.GetClusters()),
		RetentionDays:            domain.Configuration.GetWorkflowExecutionRetentionPeriodInDays(),
		EmitMetrics:              domain.Configuration.GetEmitMetric(),
		HistoryArchivalStatus:    domain.Configuration.GetHistoryArchivalStatus(),
		HistoryArchivalURI:       domain.Configuration.GetHistoryArchivalURI(),
		VisibilityArchivalStatus: domain.Configuration.GetVisibilityArchivalStatus(),
		VisibilityArchivalURI:    domain.Configuration.GetVisibilityArchivalURI(),
		BadBinaries:              newBadBinaryRows(domain.Configuration.BadBinaries),
		FailoverInfo:             newFailoverInfoRow(domain.FailoverInfo),
		IsActiveActiveDomain:     domain.ReplicationConfiguration.GetActiveClusters() != nil,
		ActiveClustersByRegion:   newActiveClustersByRegion(domain.ReplicationConfiguration.GetActiveClusters()),
	}
}

func newActiveClustersByRegion(activeClusters *types.ActiveClusters) []ActiveClusterInfoRow {
	if activeClusters == nil {
		return nil
	}
	rows := []ActiveClusterInfoRow{}
	for region, cluster := range activeClusters.GetActiveClustersByRegion() {
		rows = append(rows, ActiveClusterInfoRow{
			Region:          region,
			ClusterName:     cluster.ActiveClusterName,
			FailoverVersion: cluster.FailoverVersion,
		})
	}
	return rows
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

func (d *domainCLIImpl) describeDomain(
	ctx context.Context,
	request *types.DescribeDomainRequest,
) (*types.DescribeDomainResponse, error) {

	if d.frontendClient != nil {
		return d.frontendClient.DescribeDomain(ctx, request)
	}

	return d.domainHandler.DescribeDomain(ctx, request)
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

func parseActiveClustersByRegion(c *cli.Context, isActiveActiveDomain bool) (map[string]string, error) {
	var activeClustersByRegion map[string]string
	if c.IsSet(FlagActiveClustersByRegion) {
		if !isActiveActiveDomain {
			return nil, commoncli.Problem("Option --active_clusters_by_region is only supported for active-active domain. Use --active_cluster instead.", nil)
		}

		activeClustersByRegion = make(map[string]string)
		for _, regionCluster := range c.StringSlice(FlagActiveClustersByRegion) {
			splitted := strings.Split(regionCluster, ":")
			if len(splitted) != 2 {
				return nil, commoncli.Problem(fmt.Sprintf("Option --%s format is invalid. Expected format is 'region1:cluster1,region2:cluster2'", FlagActiveClustersByRegion), nil)
			}
			region, cluster := strings.TrimSpace(splitted[0]), strings.TrimSpace(splitted[1])
			activeClustersByRegion[region] = cluster
		}
	}

	if isActiveActiveDomain && len(activeClustersByRegion) == 0 {
		return nil, commoncli.Problem("Option --active_clusters_by_region is required for active-active domain.", nil)
	}

	return activeClustersByRegion, nil
}
