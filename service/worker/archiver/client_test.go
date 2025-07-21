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

package archiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/cadence/mocks"
	"golang.org/x/time/rate"

	carchiver "github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	mmocks "github.com/uber/cadence/common/metrics/mocks"
)

type clientSuite struct {
	*require.Assertions
	suite.Suite

	archiverProvider   *provider.MockArchiverProvider
	historyArchiver    *carchiver.HistoryArchiverMock
	visibilityArchiver *carchiver.VisibilityArchiverMock
	metricsClient      *mmocks.Client
	metricsScope       *mmocks.Scope
	cadenceClient      *mocks.Client
	client             *client
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(clientSuite))
}

func (s *clientSuite) SetupTest() {
	s.Assertions = require.New(s.T())
	s.archiverProvider = &provider.MockArchiverProvider{}
	s.historyArchiver = &carchiver.HistoryArchiverMock{}
	s.visibilityArchiver = &carchiver.VisibilityArchiverMock{}
	s.metricsClient = &mmocks.Client{}
	s.metricsScope = &mmocks.Scope{}
	s.cadenceClient = &mocks.Client{}
	s.metricsClient.On("Scope", metrics.ArchiverClientScope, mock.Anything).Return(s.metricsScope).Once()
	s.client = NewClient(
		s.metricsClient,
		log.NewNoop(),
		nil,
		dynamicproperties.GetIntPropertyFn(1000),
		clock.NewRatelimiter(rate.Limit(1000), 1),
		clock.NewRatelimiter(rate.Limit(1), 1),
		clock.NewRatelimiter(rate.Limit(1), 1),
		s.archiverProvider,
		dynamicproperties.GetBoolPropertyFn(false),
	).(*client)
	s.client.cadenceClient = s.cadenceClient
}

func (s *clientSuite) TearDownTest() {
	s.archiverProvider.AssertExpectations(s.T())
	s.historyArchiver.AssertExpectations(s.T())
	s.visibilityArchiver.AssertExpectations(s.T())
	s.metricsClient.AssertExpectations(s.T())
	s.metricsScope.AssertExpectations(s.T())
}

func (s *clientSuite) TestArchiveVisibilityInlineSuccess() {
	scopeDomain := &mmocks.Scope{}
	s.archiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.visibilityArchiver, nil).Once()
	s.visibilityArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	s.metricsScope.On("Tagged", mock.Anything).Return(scopeDomain)
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityInlineArchiveAttemptCountPerDomain).Once()
	resp, err := s.client.Archive(context.Background(), &ClientRequest{
		ArchiveRequest: &ArchiveRequest{
			VisibilityURI: "test:///visibility/archival",
			Targets:       []ArchivalTarget{ArchiveTargetVisibility},
			DomainName:    "test_domain_name",
		},
		AttemptArchiveInline: true,
	})
	s.NoError(err)
	s.NotNil(resp)
	s.False(resp.HistoryArchivedInline)
}

func (s *clientSuite) TestArchiveVisibilityInlineThrottled() {
	scopeDomain := &mmocks.Scope{}
	s.archiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.visibilityArchiver, nil).Once()
	s.visibilityArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	s.metricsScope.On("Tagged", mock.Anything).Return(scopeDomain)
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityRequestCountPerDomain).Times(2)
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityInlineArchiveAttemptCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityInlineArchiveThrottledCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientSendSignalCountPerDomain).Once()
	s.cadenceClient.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(v ArchiveRequest) bool {
		return len(v.Targets) == 1 && v.Targets[0] == ArchiveTargetVisibility
	}), mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	for i := 0; i < 2; i++ {
		resp, err := s.client.Archive(context.Background(), &ClientRequest{
			ArchiveRequest: &ArchiveRequest{
				VisibilityURI: "test:///visibility/archival",
				Targets:       []ArchivalTarget{ArchiveTargetVisibility},
				DomainName:    "test_domain_name",
			},
			AttemptArchiveInline: true,
		})
		s.NoError(err)
		s.NotNil(resp)
	}
}

func (s *clientSuite) TestArchiveVisibilityInlineFail_SendSignalSuccess() {
	scopeDomain := &mmocks.Scope{}
	s.archiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.visibilityArchiver, nil).Once()
	s.visibilityArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("some random error")).Once()
	s.metricsScope.On("Tagged", mock.Anything).Return(scopeDomain)
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityInlineArchiveAttemptCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityInlineArchiveFailureCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientSendSignalCountPerDomain).Once()
	s.cadenceClient.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(v ArchiveRequest) bool {
		return len(v.Targets) == 1 && v.Targets[0] == ArchiveTargetVisibility
	}), mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	resp, err := s.client.Archive(context.Background(), &ClientRequest{
		ArchiveRequest: &ArchiveRequest{
			VisibilityURI: "test:///visibility/archival",
			Targets:       []ArchivalTarget{ArchiveTargetVisibility},
			DomainName:    "test_domain_name",
		},
		AttemptArchiveInline: true,
	})
	s.NoError(err)
	s.NotNil(resp)
	s.False(resp.HistoryArchivedInline)
}

func (s *clientSuite) TestArchiveVisibilityInlineFail_SendSignalFail() {
	scopeDomain := &mmocks.Scope{}
	s.archiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.visibilityArchiver, nil).Once()
	s.visibilityArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("some random error")).Once()
	s.metricsScope.On("Tagged", mock.Anything).Return(scopeDomain)
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityInlineArchiveAttemptCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityInlineArchiveFailureCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientSendSignalCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientSendSignalFailureCountPerDomain).Once()
	s.cadenceClient.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(v ArchiveRequest) bool {
		return len(v.Targets) == 1 && v.Targets[0] == ArchiveTargetVisibility
	}), mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("some random error"))

	resp, err := s.client.Archive(context.Background(), &ClientRequest{
		ArchiveRequest: &ArchiveRequest{
			VisibilityURI: "test:///visibility/archival",
			Targets:       []ArchivalTarget{ArchiveTargetVisibility},
			DomainName:    "test_domain_name",
		},
		AttemptArchiveInline: true,
	})
	s.Error(err)
	s.Nil(resp)
}

func (s *clientSuite) TestArchiveHistoryInlineSuccess() {
	scopeDomain := &mmocks.Scope{}
	s.archiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.historyArchiver, nil).Once()
	s.historyArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	s.metricsScope.On("Tagged", mock.Anything).Return(scopeDomain)
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryInlineArchiveAttemptCountPerDomain).Once()
	resp, err := s.client.Archive(context.Background(), &ClientRequest{
		ArchiveRequest: &ArchiveRequest{
			URI:        "test:///history/archival",
			Targets:    []ArchivalTarget{ArchiveTargetHistory},
			DomainName: "test_domain_name",
		},
		AttemptArchiveInline: true,
	})
	s.NoError(err)
	s.NotNil(resp)
	s.True(resp.HistoryArchivedInline)
}

func (s *clientSuite) TestArchiveHistoryInlineThrottled() {
	scopeDomain := &mmocks.Scope{}
	s.archiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.historyArchiver, nil).Once()
	s.historyArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	s.metricsScope.On("Tagged", mock.Anything).Return(scopeDomain)
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryRequestCountPerDomain).Times(2)
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryInlineArchiveAttemptCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryInlineArchiveThrottledCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientSendSignalCountPerDomain).Once()
	s.cadenceClient.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(v ArchiveRequest) bool {
		return len(v.Targets) == 1 && v.Targets[0] == ArchiveTargetHistory
	}), mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	for i := 0; i < 2; i++ {
		resp, err := s.client.Archive(context.Background(), &ClientRequest{
			ArchiveRequest: &ArchiveRequest{
				URI:        "test:///history/archival",
				Targets:    []ArchivalTarget{ArchiveTargetHistory},
				DomainName: "test_domain_name",
			},
			AttemptArchiveInline: true,
		})
		s.NoError(err)
		s.NotNil(resp)
	}
}

func (s *clientSuite) TestArchiveHistoryInlineFail_SendSignalSuccess() {
	scopeDomain := &mmocks.Scope{}
	s.archiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.historyArchiver, nil).Once()
	s.historyArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("some random error")).Once()
	s.metricsScope.On("Tagged", mock.Anything).Return(scopeDomain)
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryInlineArchiveAttemptCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryInlineArchiveFailureCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientSendSignalCountPerDomain).Once()
	s.cadenceClient.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(v ArchiveRequest) bool {
		return len(v.Targets) == 1 && v.Targets[0] == ArchiveTargetHistory
	}), mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	resp, err := s.client.Archive(context.Background(), &ClientRequest{
		ArchiveRequest: &ArchiveRequest{
			URI:        "test:///history/archival",
			Targets:    []ArchivalTarget{ArchiveTargetHistory},
			DomainName: "test_domain_name",
		},
		AttemptArchiveInline: true,
	})
	s.NoError(err)
	s.NotNil(resp)
	s.False(resp.HistoryArchivedInline)
}

func (s *clientSuite) TestArchiveHistoryInlineFail_SendSignalFail() {
	scopeDomain := &mmocks.Scope{}
	s.archiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.historyArchiver, nil).Once()
	s.historyArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("some random error")).Once()
	s.metricsScope.On("Tagged", mock.Anything).Return(scopeDomain)
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryInlineArchiveAttemptCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryInlineArchiveFailureCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientSendSignalCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientSendSignalFailureCountPerDomain).Once()
	s.cadenceClient.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(v ArchiveRequest) bool {
		return len(v.Targets) == 1 && v.Targets[0] == ArchiveTargetHistory
	}), mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("some random error"))

	resp, err := s.client.Archive(context.Background(), &ClientRequest{
		ArchiveRequest: &ArchiveRequest{
			URI:        "test:///history/archival",
			Targets:    []ArchivalTarget{ArchiveTargetHistory},
			DomainName: "test_domain_name",
		},
		AttemptArchiveInline: true,
	})
	s.Error(err)
	s.Nil(resp)
}

func (s *clientSuite) TestArchiveInline_HistoryFail_VisibilitySuccess() {
	scopeDomain := &mmocks.Scope{}
	s.archiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.historyArchiver, nil).Once()
	s.archiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.visibilityArchiver, nil).Once()
	s.historyArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("some random error")).Once()
	s.visibilityArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	s.metricsScope.On("Tagged", mock.Anything).Return(scopeDomain)
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryInlineArchiveAttemptCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryInlineArchiveFailureCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityInlineArchiveAttemptCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientSendSignalCountPerDomain).Once()
	s.cadenceClient.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(v ArchiveRequest) bool {
		return len(v.Targets) == 1 && v.Targets[0] == ArchiveTargetHistory
	}), mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	resp, err := s.client.Archive(context.Background(), &ClientRequest{
		ArchiveRequest: &ArchiveRequest{
			URI:           "test:///history/archival",
			VisibilityURI: "test:///visibility/archival",
			Targets:       []ArchivalTarget{ArchiveTargetHistory, ArchiveTargetVisibility},
			DomainName:    "test_domain_name",
		},
		AttemptArchiveInline: true,
	})
	s.NoError(err)
	s.NotNil(resp)
	s.False(resp.HistoryArchivedInline)
}

func (s *clientSuite) TestArchiveInline_VisibilityFail_HistorySuccess() {
	scopeDomain := &mmocks.Scope{}
	s.archiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.historyArchiver, nil).Once()
	s.archiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.visibilityArchiver, nil).Once()
	s.historyArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	s.visibilityArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("some random error")).Once()
	s.metricsScope.On("Tagged", mock.Anything).Return(scopeDomain)
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryInlineArchiveAttemptCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityInlineArchiveAttemptCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityInlineArchiveFailureCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientSendSignalCountPerDomain).Once()
	s.cadenceClient.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(v ArchiveRequest) bool {
		return len(v.Targets) == 1 && v.Targets[0] == ArchiveTargetVisibility
	}), mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	resp, err := s.client.Archive(context.Background(), &ClientRequest{
		ArchiveRequest: &ArchiveRequest{
			URI:           "test:///history/archival",
			VisibilityURI: "test:///visibility/archival",
			Targets:       []ArchivalTarget{ArchiveTargetHistory, ArchiveTargetVisibility},
			DomainName:    "test_domain_name",
		},
		AttemptArchiveInline: true,
	})
	s.NoError(err)
	s.NotNil(resp)
	s.True(resp.HistoryArchivedInline)
}

func (s *clientSuite) TestArchiveInline_VisibilityFail_HistoryFail() {
	scopeDomain := &mmocks.Scope{}
	s.archiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.historyArchiver, nil).Once()
	s.archiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.visibilityArchiver, nil).Once()
	s.historyArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("some random error")).Once()
	s.visibilityArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("some random error")).Once()
	s.metricsScope.On("Tagged", mock.Anything).Return(scopeDomain)
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryInlineArchiveAttemptCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryInlineArchiveFailureCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityInlineArchiveAttemptCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityInlineArchiveFailureCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientSendSignalCountPerDomain).Once()
	s.cadenceClient.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(v ArchiveRequest) bool {
		return len(v.Targets) == 2
	}), mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	resp, err := s.client.Archive(context.Background(), &ClientRequest{
		ArchiveRequest: &ArchiveRequest{
			URI:           "test:///history/archival",
			VisibilityURI: "test:///visibility/archival",
			Targets:       []ArchivalTarget{ArchiveTargetHistory, ArchiveTargetVisibility},
			DomainName:    "test_domain_name",
		},
		AttemptArchiveInline: true,
	})
	s.NoError(err)
	s.NotNil(resp)
	s.False(resp.HistoryArchivedInline)
}

func (s *clientSuite) TestArchiveInline_VisibilitySuccess_HistorySuccess() {
	scopeDomain := &mmocks.Scope{}
	s.archiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.historyArchiver, nil).Once()
	s.archiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.visibilityArchiver, nil).Once()
	s.historyArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	s.visibilityArchiver.On("Archive", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	s.metricsScope.On("Tagged", mock.Anything).Return(scopeDomain)
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryInlineArchiveAttemptCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityInlineArchiveAttemptCountPerDomain).Once()
	resp, err := s.client.Archive(context.Background(), &ClientRequest{
		ArchiveRequest: &ArchiveRequest{
			URI:           "test:///history/archival",
			VisibilityURI: "test:///visibility/archival",
			Targets:       []ArchivalTarget{ArchiveTargetHistory, ArchiveTargetVisibility},
			DomainName:    "test_domain_name",
		},
		AttemptArchiveInline: true,
	})
	s.NoError(err)
	s.NotNil(resp)
	s.True(resp.HistoryArchivedInline)
}

func (s *clientSuite) TestArchiveSendSignal_Success() {
	scopeDomain := &mmocks.Scope{}
	s.cadenceClient.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(v ArchiveRequest) bool {
		return len(v.Targets) == 2
	}), mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	s.metricsScope.On("Tagged", mock.Anything).Return(scopeDomain)
	scopeDomain.On("IncCounter", metrics.ArchiverClientHistoryRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientVisibilityRequestCountPerDomain).Once()
	scopeDomain.On("IncCounter", metrics.ArchiverClientSendSignalCountPerDomain).Once()
	resp, err := s.client.Archive(context.Background(), &ClientRequest{
		ArchiveRequest: &ArchiveRequest{
			URI:           "test:///history/archival",
			VisibilityURI: "test:///visibility/archival",
			Targets:       []ArchivalTarget{ArchiveTargetHistory, ArchiveTargetVisibility},
			DomainName:    "test_domain_name",
		},
		AttemptArchiveInline: false,
	})
	s.NoError(err)
	s.NotNil(resp)
	s.False(resp.HistoryArchivedInline)
}

func (s *clientSuite) TestArchiveUnknownTarget() {
	s.metricsScope.On("Tagged", mock.Anything).Return(&mmocks.Scope{})
	resp, err := s.client.Archive(context.Background(), &ClientRequest{
		ArchiveRequest: &ArchiveRequest{
			Targets: []ArchivalTarget{3},
		},
		AttemptArchiveInline: true,
	})
	s.NoError(err)
	s.NotNil(resp)
	s.False(resp.HistoryArchivedInline)
}
