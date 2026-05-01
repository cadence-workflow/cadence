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

package timeout

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common/types"
)

func Test_historyClient_CloseShard(t *testing.T) {
	type fields struct {
		client  func(t *testing.T) history.Client
		timeout time.Duration
	}
	type args struct {
		ctx context.Context
		cp1 *types.CloseShardRequest
		p1  []yarpc.CallOption
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "nil context success",
			fields: fields{
				timeout: time.Millisecond * 150,
			},
			args: args{
				ctx: nil,
				cp1: &types.CloseShardRequest{},
			},
			wantErr: false,
		},
		{
			name: "context failed with timeout",
			fields: fields{
				timeout: time.Millisecond * 50,
			},
			args: args{
				ctx: context.Background(),
				cp1: &types.CloseShardRequest{},
			},
			wantErr: true,
		},
		{
			name: "invalid timeout value success",
			fields: fields{
				timeout: -10,
			},
			args: args{
				ctx: context.Background(),
				cp1: &types.CloseShardRequest{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := history.NewMockClient(gomock.NewController(t))
			m.EXPECT().CloseShard(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, r *types.CloseShardRequest, opt ...yarpc.CallOption) error {
				for {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(time.Millisecond * 100):
						return nil
					}
				}
			})
			c := historyClient{
				client:  m,
				timeout: tt.fields.timeout,
			}
			if err := c.CloseShard(tt.args.ctx, tt.args.cp1, tt.args.p1...); (err != nil) != tt.wantErr {
				t.Errorf("CloseShard() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_historyClient_GetReplicationMessages(t *testing.T) {
	t.Run("no timeout", func(t *testing.T) {
		m := history.NewMockClient(gomock.NewController(t))
		m.EXPECT().GetReplicationMessages(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, r *types.GetReplicationMessagesRequest, opt ...yarpc.CallOption) (*types.GetReplicationMessagesResponse, error) {
			for {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(time.Millisecond * 100):
					return &types.GetReplicationMessagesResponse{}, nil
				}
			}
		})
		c := historyClient{
			client:  m,
			timeout: time.Millisecond * 10,
		}
		_, err := c.GetReplicationMessages(context.Background(), &types.GetReplicationMessagesRequest{})
		assert.NoError(t, err)
	})
}
