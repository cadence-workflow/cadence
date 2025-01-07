// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJsonTaskTokenSerializer_Token_RoundTrip(t *testing.T) {
	token := TaskToken{
		DomainID:        "test-domain",
		WorkflowID:      "test-workflow-id",
		WorkflowType:    "test-workflow-type",
		RunID:           "test-run-id",
		ScheduleID:      1,
		ScheduleAttempt: 1,
		ActivityID:      "test-activity-id",
		ActivityType:    "test-activity-type",
	}

	serializer := NewJSONTaskTokenSerializer()

	serialized, err := serializer.Serialize(&token)
	require.NoError(t, err)
	deserializedToken, err := serializer.Deserialize(serialized)
	require.NoError(t, err)
	require.Equal(t, token, *deserializedToken)
}

func TestNewJSONTaskTokenSerializer_QueryToken_Roundtrip(t *testing.T) {
	token := QueryTaskToken{
		DomainID:   "test-domain",
		WorkflowID: "test-workflow-id",
		RunID:      "test-run-id",
		TaskList:   "test-task-list",
		TaskID:     "test-task-id",
	}
	serializer := NewJSONTaskTokenSerializer()

	serialized, err := serializer.SerializeQueryTaskToken(&token)
	require.NoError(t, err)
	deserializedToken, err := serializer.DeserializeQueryTaskToken(serialized)
	require.NoError(t, err)
	require.Equal(t, token, *deserializedToken)
}
