package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/matching"
)

// matchingFactory is a Factory which only creates matching clients
type matchingFactory struct {
	Factory
	client             matching.Client
	pollerDisconnector matching.PollerDisconnector
	calls              int
}

func (f *matchingFactory) NewMatchingClient(DomainIDToNameFunc) (matching.Client, matching.PollerDisconnector, error) {
	f.calls++
	return f.client, f.pollerDisconnector, nil
}

func TestMatchingClientAndPollerDisconnectorAreCreatedTogether(t *testing.T) {
	ctrl := gomock.NewController(t)
	factory := &matchingFactory{
		client:             matching.NewMockClient(ctrl),
		pollerDisconnector: matching.NewMockPollerDisconnector(ctrl),
	}
	bean := &clientBeanImpl{factory: factory}

	pollerDisconnector, err := bean.GetMatchingPollerDisconnector(nil)
	require.NoError(t, err)
	client, err := bean.GetMatchingClient(nil)
	require.NoError(t, err)

	assert.Same(t, factory.pollerDisconnector, pollerDisconnector)
	assert.Same(t, factory.client, client)
	assert.Equal(t, 1, factory.calls)
}
