package asyncworkflowqueue

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/.gen/go/sqlblobs"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
)

func mustEncodeStartInner(t *testing.T, req *types.StartWorkflowExecutionRequest) []byte {
	t.Helper()
	b, err := codec.NewThriftRWEncoder().Encode(thrift.FromStartWorkflowExecutionAsyncRequest(
		&types.StartWorkflowExecutionAsyncRequest{StartWorkflowExecutionRequest: req}))
	require.NoError(t, err)
	return b
}

func mustEncodeSignalInner(t *testing.T, req *types.SignalWithStartWorkflowExecutionRequest) []byte {
	t.Helper()
	b, err := codec.NewThriftRWEncoder().Encode(thrift.FromSignalWithStartWorkflowExecutionAsyncRequest(
		&types.SignalWithStartWorkflowExecutionAsyncRequest{SignalWithStartWorkflowExecutionRequest: req}))
	require.NoError(t, err)
	return b
}

// mustEncodeEnvelope builds the row payload: the ThriftRW-encoded sqlblobs.AsyncRequestMessage envelope.
// Pass typ == nil to leave the envelope Type unset and encoding == nil to leave the envelope Encoding unset.
func mustEncodeEnvelope(
	t *testing.T,
	typ *sqlblobs.AsyncRequestType,
	encoding *string,
	header *shared.Header,
	inner []byte,
) []byte {
	t.Helper()
	b, err := codec.NewThriftRWEncoder().Encode(&sqlblobs.AsyncRequestMessage{
		PartitionKey: common.StringPtr("wf-key"),
		Type:         typ,
		Header:       header,
		Encoding:     encoding,
		Payload:      inner,
	})
	require.NoError(t, err)
	return b
}

func TestDecoder_Decode(t *testing.T) {
	errLookup := errors.New("domain lookup failed")
	okLookup := func(name string) (string, error) {
		if name != "test-domain" {
			return "", errors.New("unexpected domain " + name)
		}
		return "domain-id-1", nil
	}

	thriftRW := common.StringPtr(string(constants.EncodingTypeThriftRW))
	jsonEnc := common.StringPtr(string(constants.EncodingTypeJSON))
	startType := sqlblobs.AsyncRequestTypeStartWorkflowExecutionAsyncRequest.Ptr()
	signalType := sqlblobs.AsyncRequestTypeSignalWithStartWorkflowExecutionAsyncRequest.Ptr()
	unknownType := sqlblobs.AsyncRequestType(99)

	headers := &shared.Header{Fields: map[string][]byte{"k1": []byte("v1"), "k2": []byte("v2")}}

	startReq := &types.StartWorkflowExecutionRequest{
		Domain:       "test-domain",
		WorkflowID:   "wf-1",
		RequestID:    "req-1",
		WorkflowType: &types.WorkflowType{Name: "wf-type"},
		TaskList:     &types.TaskList{Name: "tl"},
		Input:        []byte("input"),
	}
	signalReq := &types.SignalWithStartWorkflowExecutionRequest{
		Domain:       "test-domain",
		WorkflowID:   "wf-2",
		RequestID:    "req-2",
		WorkflowType: &types.WorkflowType{Name: "wf-type"},
		TaskList:     &types.TaskList{Name: "tl"},
		SignalName:   "sig",
		SignalInput:  []byte("sig-input"),
	}

	garbage := []byte{0xff, 0xfe, 0xfd, 0x01, 0x02}
	startInner := mustEncodeStartInner(t, startReq)
	signalInner := mustEncodeSignalInner(t, signalReq)

	// Well-formed inner payloads whose request is absent.
	emptyStartInner, err := codec.NewThriftRWEncoder().Encode(&shared.StartWorkflowExecutionAsyncRequest{})
	require.NoError(t, err)
	emptySignalInner, err := codec.NewThriftRWEncoder().Encode(&shared.SignalWithStartWorkflowExecutionAsyncRequest{})
	require.NoError(t, err)

	row := func(rt persistence.AsyncWorkflowRequestType, enc constants.EncodingType, payload []byte) *persistence.AsyncWorkflowMessage {
		return &persistence.AsyncWorkflowMessage{MessageID: 1, RequestType: rt, Payload: payload, PayloadEncoding: enc}
	}
	rowStart := persistence.AsyncWorkflowRequestTypeStartWorkflow
	rowSignal := persistence.AsyncWorkflowRequestTypeSignalWithStartWorkflow
	rw := constants.EncodingTypeThriftRW

	tests := []struct {
		name        string
		msg         *persistence.AsyncWorkflowMessage
		domainIDFn  func(string) (string, error)
		wantCorrupt bool
		wantErr     error // exact, unwrapped error expected when not corrupt
		wantStart   bool
		wantSignal  bool
		wantWFID    string
		wantReqID   string
		wantHeaders map[string][]byte
	}{
		{
			name:        "nil message is corrupt, not a panic",
			msg:         nil,
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:        "start workflow happy path, headers copied through",
			msg:         row(rowStart, rw, mustEncodeEnvelope(t, startType, thriftRW, headers, startInner)),
			domainIDFn:  okLookup,
			wantStart:   true,
			wantWFID:    "wf-1",
			wantReqID:   "req-1",
			wantHeaders: map[string][]byte{"k1": []byte("v1"), "k2": []byte("v2")},
		},
		{
			name:        "signal with start happy path, headers copied through",
			msg:         row(rowSignal, rw, mustEncodeEnvelope(t, signalType, thriftRW, headers, signalInner)),
			domainIDFn:  okLookup,
			wantSignal:  true,
			wantWFID:    "wf-2",
			wantReqID:   "req-2",
			wantHeaders: map[string][]byte{"k1": []byte("v1"), "k2": []byte("v2")},
		},
		{
			name:       "envelope without header yields nil Headers",
			msg:        row(rowStart, rw, mustEncodeEnvelope(t, startType, thriftRW, nil, startInner)),
			domainIDFn: okLookup,
			wantStart:  true,
			wantWFID:   "wf-1",
			wantReqID:  "req-1",
		},
		{
			name:       "envelope with header but no fields yields nil Headers",
			msg:        row(rowSignal, rw, mustEncodeEnvelope(t, signalType, thriftRW, &shared.Header{}, signalInner)),
			domainIDFn: okLookup,
			wantSignal: true,
			wantWFID:   "wf-2",
			wantReqID:  "req-2",
		},
		{
			name:        "row payload encoding is not thriftrw",
			msg:         row(rowStart, constants.EncodingTypeJSON, mustEncodeEnvelope(t, startType, thriftRW, headers, startInner)),
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:        "row payload encoding is empty",
			msg:         row(rowStart, constants.EncodingTypeEmpty, mustEncodeEnvelope(t, startType, thriftRW, headers, startInner)),
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:        "envelope decode failure (garbage row payload), start",
			msg:         row(rowStart, rw, garbage),
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:        "envelope decode failure (garbage row payload), signal with start",
			msg:         row(rowSignal, rw, garbage),
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:        "envelope type signal-with-start but row type start",
			msg:         row(rowStart, rw, mustEncodeEnvelope(t, signalType, thriftRW, headers, signalInner)),
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:        "envelope type start but row type signal-with-start",
			msg:         row(rowSignal, rw, mustEncodeEnvelope(t, startType, thriftRW, headers, startInner)),
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:        "envelope type unknown",
			msg:         row(rowStart, rw, mustEncodeEnvelope(t, &unknownType, thriftRW, headers, startInner)),
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:        "envelope type absent",
			msg:         row(rowStart, rw, mustEncodeEnvelope(t, nil, thriftRW, headers, startInner)),
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:        "row request type unknown",
			msg:         row(persistence.AsyncWorkflowRequestType(99), rw, mustEncodeEnvelope(t, startType, thriftRW, headers, startInner)),
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:        "envelope encoding is not thriftrw",
			msg:         row(rowStart, rw, mustEncodeEnvelope(t, startType, jsonEnc, headers, startInner)),
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:        "envelope encoding absent",
			msg:         row(rowStart, rw, mustEncodeEnvelope(t, startType, nil, headers, startInner)),
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:        "inner decode failure, start",
			msg:         row(rowStart, rw, mustEncodeEnvelope(t, startType, thriftRW, headers, garbage)),
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:        "inner decode failure, signal with start",
			msg:         row(rowSignal, rw, mustEncodeEnvelope(t, signalType, thriftRW, headers, garbage)),
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:        "start with nil inner request",
			msg:         row(rowStart, rw, mustEncodeEnvelope(t, startType, thriftRW, headers, emptyStartInner)),
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:        "signal with start with nil inner request",
			msg:         row(rowSignal, rw, mustEncodeEnvelope(t, signalType, thriftRW, headers, emptySignalInner)),
			domainIDFn:  okLookup,
			wantCorrupt: true,
		},
		{
			name:       "domain lookup error is returned unwrapped and is not corrupt",
			msg:        row(rowStart, rw, mustEncodeEnvelope(t, startType, thriftRW, headers, startInner)),
			domainIDFn: func(string) (string, error) { return "", errLookup },
			wantErr:    errLookup,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewDecoder(tc.domainIDFn).Decode(tc.msg)

			if tc.wantCorrupt {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrCorruptMessage)
				require.Nil(t, got)
				return
			}
			if tc.wantErr != nil {
				require.Equal(t, tc.wantErr, err, "domain lookup error must be returned unwrapped")
				require.NotErrorIs(t, err, ErrCorruptMessage)
				require.Nil(t, got)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			require.Same(t, tc.msg, got.Message)
			require.Equal(t, "domain-id-1", got.DomainID)
			require.Empty(t, got.ActiveCluster)
			require.Equal(t, tc.wantHeaders, got.Headers)
			if tc.wantStart {
				require.Nil(t, got.SignalWithStart)
				require.NotNil(t, got.Start)
				require.Equal(t, tc.wantWFID, got.Start.WorkflowID)
				require.Equal(t, tc.wantReqID, got.Start.RequestID)
				require.Equal(t, "test-domain", got.Start.Domain)
				require.Equal(t, "wf-type", got.Start.WorkflowType.Name)
			}
			if tc.wantSignal {
				require.Nil(t, got.Start)
				require.NotNil(t, got.SignalWithStart)
				require.Equal(t, tc.wantWFID, got.SignalWithStart.WorkflowID)
				require.Equal(t, tc.wantReqID, got.SignalWithStart.RequestID)
				require.Equal(t, "sig", got.SignalWithStart.SignalName)
			}
		})
	}
}

func TestDecoder_Decode_HeadersAreCopied(t *testing.T) {
	// Mutating the decoded Headers must not be able to corrupt anything shared with the row: Decode copies.
	header := &shared.Header{Fields: map[string][]byte{"k1": []byte("v1")}}
	startInner := mustEncodeStartInner(t, &types.StartWorkflowExecutionRequest{Domain: "test-domain", WorkflowID: "wf-1"})
	msg := &persistence.AsyncWorkflowMessage{
		RequestType:     persistence.AsyncWorkflowRequestTypeStartWorkflow,
		Payload:         mustEncodeEnvelope(t, sqlblobs.AsyncRequestTypeStartWorkflowExecutionAsyncRequest.Ptr(), common.StringPtr(string(constants.EncodingTypeThriftRW)), header, startInner),
		PayloadEncoding: constants.EncodingTypeThriftRW,
	}
	dec := NewDecoder(func(string) (string, error) { return "domain-id-1", nil })

	first, err := dec.Decode(msg)
	require.NoError(t, err)
	first.Headers["k1"] = []byte("mutated")
	first.Headers["extra"] = []byte("x")

	second, err := dec.Decode(msg)
	require.NoError(t, err)
	require.Equal(t, map[string][]byte{"k1": []byte("v1")}, second.Headers)
}
