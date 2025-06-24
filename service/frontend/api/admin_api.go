//Admin API Interface

package api

import (
   "context"
   "fmt"
   "github.com/uber/cadence/common/types"
)

type Handler interface {
    StartDomainDiagnosisWorkflow (context.Context, *types.StartDomainDiagnosisWorkflowRequest) (*types.StartDomainDiagnosisWorkflowResponse, error)
    FetchDomainDiagnosisActivity(context.Context, *types.FetchDomainDiagnosisActivityRequest) (*types.FetchDomainDiagnosisActivityResponse, error)
}
