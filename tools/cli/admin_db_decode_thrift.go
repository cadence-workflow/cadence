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
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/.gen/go/config"
	"github.com/uber/cadence/.gen/go/history"
	"github.com/uber/cadence/.gen/go/replicator"
	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/.gen/go/sqlblobs"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/tools/common/commoncli"
)

var decodingTypes = map[string]func() codec.ThriftObject{
	"shared.History":                    func() codec.ThriftObject { return &shared.History{} },
	"shared.HistoryEvent":               func() codec.ThriftObject { return &shared.HistoryEvent{} },
	"shared.HistoryBranch":              func() codec.ThriftObject { return &shared.HistoryBranch{} },
	"shared.Memo":                       func() codec.ThriftObject { return &shared.Memo{} },
	"shared.ResetPoints":                func() codec.ThriftObject { return &shared.ResetPoints{} },
	"shared.BadBinaries":                func() codec.ThriftObject { return &shared.BadBinaries{} },
	"shared.VersionHistories":           func() codec.ThriftObject { return &shared.VersionHistories{} },
	"replicator.FailoverMarkers":        func() codec.ThriftObject { return &replicator.FailoverMarkers{} },
	"history.ProcessingQueueStates":     func() codec.ThriftObject { return &history.ProcessingQueueStates{} },
	"config.DynamicConfigBlob":          func() codec.ThriftObject { return &config.DynamicConfigBlob{} },
	"sqlblobs.ShardInfo":                func() codec.ThriftObject { return &sqlblobs.ShardInfo{} },
	"sqlblobs.DomainInfo":               func() codec.ThriftObject { return &sqlblobs.DomainInfo{} },
	"sqlblobs.HistoryTreeInfo":          func() codec.ThriftObject { return &sqlblobs.HistoryTreeInfo{} },
	"sqlblobs.WorkflowExecutionInfo":    func() codec.ThriftObject { return &sqlblobs.WorkflowExecutionInfo{} },
	"sqlblobs.ActivityInfo":             func() codec.ThriftObject { return &sqlblobs.ActivityInfo{} },
	"sqlblobs.ChildExecutionInfo":       func() codec.ThriftObject { return &sqlblobs.ChildExecutionInfo{} },
	"sqlblobs.SignalInfo":               func() codec.ThriftObject { return &sqlblobs.SignalInfo{} },
	"sqlblobs.RequestCancelInfo":        func() codec.ThriftObject { return &sqlblobs.RequestCancelInfo{} },
	"sqlblobs.TimerInfo":                func() codec.ThriftObject { return &sqlblobs.TimerInfo{} },
	"sqlblobs.TaskInfo":                 func() codec.ThriftObject { return &sqlblobs.TaskInfo{} },
	"sqlblobs.TaskListInfo":             func() codec.ThriftObject { return &sqlblobs.TaskListInfo{} },
	"sqlblobs.TransferTaskInfo":         func() codec.ThriftObject { return &sqlblobs.TransferTaskInfo{} },
	"sqlblobs.TimerTaskInfo":            func() codec.ThriftObject { return &sqlblobs.TimerTaskInfo{} },
	"sqlblobs.ReplicationTaskInfo":      func() codec.ThriftObject { return &sqlblobs.ReplicationTaskInfo{} },
	"shared.AsyncWorkflowConfiguration": func() codec.ThriftObject { return &shared.AsyncWorkflowConfiguration{} },
}

type decodeError struct {
	shortMsg string
	err      error
}

// AdminDBDataDecodeThrift is the command to decode thrift binary into JSON
func AdminDBDataDecodeThrift(c *cli.Context) error {
	input, err := getRequiredOption(c, FlagInput)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	encoding := c.String(FlagInputEncoding)
	data, err := decodeUserInput(input, encoding)
	if err != nil {
		return commoncli.Problem("failed to decode input", err)
	}

	if _, err := decodeThriftPayload(data); err != nil {
		return commoncli.Problem("failed to decode thrift payload", err.err)
	}
	return nil
}

func decodeThriftPayload(data []byte) (codec.ThriftObject, *decodeError) {
	encoder := codec.NewThriftRWEncoder()
	// this is an inconsistency in the code base, some place use ThriftRWEncoder(version0Thriftrw.go) some use thriftEncoder(thrift_encoder.go)
	dataWithPrepend := []byte{0x59}
	dataWithPrepend = append(dataWithPrepend, data...)
	datas := [][]byte{data, dataWithPrepend}

	for _, data := range datas {
		for typeName, objFn := range decodingTypes {
			t := objFn()
			if err := encoder.Decode(data, t); err != nil {
				continue
			}

			// encoding back to confirm
			data2, err := encoder.Encode(t)
			if err != nil {
				return nil, &decodeError{
					shortMsg: "cannot encode back to confirm",
					err:      err,
				}
			}
			if !bytes.Equal(data, data2) {
				continue
			}

			fmt.Printf("======= Decode into type %v ========\n", typeName)
			spew.Dump(t)
			// json-ify it for easier mechanical use
			js, err := json.Marshal(t)
			if err == nil {
				fmt.Println("======= As JSON ========")
				fmt.Println(string(js))
			}
			return t, nil
		}
	}

	return nil, &decodeError{
		shortMsg: "input data cannot be decoded into any struct",
		err:      nil,
	}
}

func decodeUserInput(input, encoding string) ([]byte, error) {
	switch encoding {
	case "", "hex":
		// remove "0x" from the beginning of the input. hex library doesn't expect it but that's how it's printed it out by csql
		input = strings.TrimPrefix(input, "0x")
		return hex.DecodeString(input)
	case "base64":
		return base64.StdEncoding.DecodeString(input)
	}

	return nil, fmt.Errorf("unknown input encoding: %s", encoding)
}
