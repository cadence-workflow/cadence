package asyncworkflowqueue

import (
	"fmt"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/.gen/go/sqlblobs"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types/mapper/thrift"
)

// Decoder converts a queue row into a typed request.
type Decoder interface {
	Decode(msg *persistence.AsyncWorkflowMessage) (*Request, error) // ErrCorruptMessage (wrapped) for bad row/envelope encoding, type mismatch, bad bytes, nil inner; raw error from domainIDFn
}

type decoder struct {
	domainIDFn func(domainName string) (string, error)
	enc        *codec.ThriftRWEncoder
}

// NewDecoder returns a Decoder that resolves domain IDs through domainIDFn.
func NewDecoder(domainIDFn func(domainName string) (string, error)) Decoder {
	return &decoder{domainIDFn: domainIDFn, enc: codec.NewThriftRWEncoder()}
}

func (d *decoder) Decode(msg *persistence.AsyncWorkflowMessage) (*Request, error) {
	if msg == nil {
		return nil, fmt.Errorf("%w: message is nil", ErrCorruptMessage)
	}
	if msg.PayloadEncoding != constants.EncodingTypeThriftRW {
		return nil, fmt.Errorf("%w: unsupported row payload encoding %q", ErrCorruptMessage, msg.PayloadEncoding)
	}

	var env sqlblobs.AsyncRequestMessage
	if err := d.enc.Decode(msg.Payload, &env); err != nil {
		return nil, fmt.Errorf("%w: decode envelope: %v", ErrCorruptMessage, err)
	}

	if env.Type == nil {
		return nil, fmt.Errorf("%w: envelope type is absent", ErrCorruptMessage)
	}
	envType := env.GetType()
	isStart := envType == sqlblobs.AsyncRequestTypeStartWorkflowExecutionAsyncRequest &&
		msg.RequestType == persistence.AsyncWorkflowRequestTypeStartWorkflow
	isSignal := envType == sqlblobs.AsyncRequestTypeSignalWithStartWorkflowExecutionAsyncRequest &&
		msg.RequestType == persistence.AsyncWorkflowRequestTypeSignalWithStartWorkflow
	if !isStart && !isSignal {
		return nil, fmt.Errorf("%w: envelope type %v does not match row request type %d", ErrCorruptMessage, envType, msg.RequestType)
	}

	if env.GetEncoding() != string(constants.EncodingTypeThriftRW) {
		return nil, fmt.Errorf("%w: unsupported envelope encoding %q", ErrCorruptMessage, env.GetEncoding())
	}

	req := &Request{Message: msg}
	var domainName string
	if isStart {
		var obj shared.StartWorkflowExecutionAsyncRequest
		if err := d.enc.Decode(env.GetPayload(), &obj); err != nil {
			return nil, fmt.Errorf("%w: decode start workflow request: %v", ErrCorruptMessage, err)
		}
		inner := thrift.ToStartWorkflowExecutionAsyncRequest(&obj).StartWorkflowExecutionRequest
		if inner == nil {
			return nil, fmt.Errorf("%w: start workflow request is empty", ErrCorruptMessage)
		}
		req.Start = inner
		domainName = inner.GetDomain()
	} else {
		var obj shared.SignalWithStartWorkflowExecutionAsyncRequest
		if err := d.enc.Decode(env.GetPayload(), &obj); err != nil {
			return nil, fmt.Errorf("%w: decode signal with start workflow request: %v", ErrCorruptMessage, err)
		}
		inner := thrift.ToSignalWithStartWorkflowExecutionAsyncRequest(&obj).SignalWithStartWorkflowExecutionRequest
		if inner == nil {
			return nil, fmt.Errorf("%w: signal with start workflow request is empty", ErrCorruptMessage)
		}
		req.SignalWithStart = inner
		domainName = inner.GetDomain()
	}

	if fields := env.GetHeader().GetFields(); len(fields) > 0 {
		headers := make(map[string][]byte, len(fields))
		for k, v := range fields {
			headers[k] = append([]byte(nil), v...)
		}
		req.Headers = headers
	}

	domainID, err := d.domainIDFn(domainName)
	if err != nil {
		return nil, err
	}
	req.DomainID = domainID
	return req, nil
}
