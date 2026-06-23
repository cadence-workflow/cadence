package configstore

import (
	"github.com/uber/cadence/common/dynamicconfig"
)

// NewNopClient returns a Client whose reads return defaults and whose writes return "not supported".
func NewNopClient() Client {
	return &nopConfigStoreClient{Client: dynamicconfig.NewNopClient()}
}

type nopConfigStoreClient struct {
	dynamicconfig.Client
}

func (c *nopConfigStoreClient) Start() {}
func (c *nopConfigStoreClient) Stop()  {}
