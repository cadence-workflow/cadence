// Admin API Interface

package api

import (
	"context"

	"github.com/uber/cadence/common/types"
)

type AdminHandler interface {
	StartDomainDiagnosisWorkflow(context.Context, *types.StartDomainDiagnosisWorkflowRequest) (*types.StartDomainDiagnosisWorkflowResponse, error)
	FetchDomainDiagnosisActivity(context.Context, *types.FetchDomainDiagnosisActivityRequest) (*types.FetchDomainDiagnosisActivityResponse, error)
}
