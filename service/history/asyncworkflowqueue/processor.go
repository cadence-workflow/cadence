package asyncworkflowqueue

import "context"

//go:generate mockgen -package $GOPACKAGE -destination processor_mock.go github.com/uber/cadence/service/history/asyncworkflowqueue Processor,Decoder

// Processor handles one decoded async workflow queue request.
type Processor interface {
	// Process makes one attempt. nil = handled (started, already started, or forwarded).
	Process(ctx context.Context, req *Request) error
}
