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

package provider

import (
	"context"

	"github.com/uber/cadence/common/archiver"
)

type noOpArchiverProvider struct{}

func NewNoOpArchiverProvider() ArchiverProvider {
	return &noOpArchiverProvider{}
}

func (*noOpArchiverProvider) RegisterBootstrapContainer(
	serviceName string,
	historyContainer *archiver.HistoryBootstrapContainer,
	visibilityContainter *archiver.VisibilityBootstrapContainer,
	executionContainer *archiver.ExecutionBootstrapContainer,
) error {
	return nil
}

func (*noOpArchiverProvider) GetHistoryArchiver(scheme, serviceName string) (archiver.HistoryArchiver, error) {
	return &noOpHistoryArchiver{}, nil
}

func (*noOpArchiverProvider) GetVisibilityArchiver(scheme, serviceName string) (archiver.VisibilityArchiver, error) {
	return &noOpVisibilityArchiver{}, nil
}

func (*noOpArchiverProvider) GetExecutionArchiver(scheme, serviceName string) (archiver.ExecutionArchiver, error) {
	return nil, nil
}

type noOpHistoryArchiver struct{}

func (*noOpHistoryArchiver) Archive(context.Context, archiver.URI, *archiver.ArchiveHistoryRequest, ...archiver.ArchiveOption) error {
	return nil
}

func (*noOpHistoryArchiver) Get(context.Context, archiver.URI, *archiver.GetHistoryRequest) (*archiver.GetHistoryResponse, error) {
	return &archiver.GetHistoryResponse{}, nil
}

func (*noOpHistoryArchiver) GetWorkflowHistoryForPersistence(context.Context, *archiver.GetHistoryRequest) (*archiver.GetHistoryResponse, error) {
	return &archiver.GetHistoryResponse{}, nil
}

func (*noOpHistoryArchiver) ValidateURI(archiver.URI) error {
	return nil
}

type noOpVisibilityArchiver struct{}

func (*noOpVisibilityArchiver) Archive(context.Context, archiver.URI, *archiver.ArchiveVisibilityRequest, ...archiver.ArchiveOption) error {
	return nil
}

func (*noOpVisibilityArchiver) Query(context.Context, archiver.URI, *archiver.QueryVisibilityRequest) (*archiver.QueryVisibilityResponse, error) {
	return &archiver.QueryVisibilityResponse{}, nil
}

func (*noOpVisibilityArchiver) ValidateURI(archiver.URI) error {
	return nil
}
