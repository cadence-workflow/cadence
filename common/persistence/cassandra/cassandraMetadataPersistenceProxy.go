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

package cassandra

import (
	"errors"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service/config"
)

type (
	// TODO, we should migrate the non global domain to new table, see #773
	// WARN this struct should only be used by the domain cache ONLY
	metadataManagerProxy struct {
		metadataMgr   p.MetadataStore
		metadataMgrV2 p.MetadataStore
		logger        log.Logger
	}
)

// newMetadataManagerProxy is used for merging the functionality the v1 and v2 MetadataManager
func newMetadataManagerProxy(cfg config.Cassandra,
	currentClusterName string, logger log.Logger) (p.MetadataStore, error) {
	metadataMgr, err := newMetadataPersistence(cfg, currentClusterName, logger)
	if err != nil {
		return nil, err
	}
	metadataMgrV2, err := newMetadataPersistenceV2(cfg, currentClusterName, logger)
	if err != nil {
		return nil, err
	}
	return &metadataManagerProxy{metadataMgr: metadataMgr, metadataMgrV2: metadataMgrV2, logger: logger}, nil
}

func (m *metadataManagerProxy) GetName() string {
	return cassandraPersistenceName
}

func (m *metadataManagerProxy) GetDomain(request *p.GetDomainRequest) (*p.GetDomainResponse, error) {
	// the reason this function does not call the v2 get domain is domain cache will
	// use the list domain function to get all domain in the v2 table
	resp, err := m.metadataMgrV2.GetDomain(request)
	if err != nil {
		if _, ok := err.(*shared.EntityNotExistsError); !ok {
			return nil, err
		}
	} else {
		resp.TableVersion = p.DomainTableVersionV2
		return resp, nil
	}

	resp, err = m.metadataMgr.GetDomain(request)
	if err == nil {
		resp.TableVersion = p.DomainTableVersionV1
	}
	return resp, err
}

func (m *metadataManagerProxy) ListDomains(request *p.ListDomainsRequest) (*p.ListDomainsResponse, error) {
	return m.metadataMgrV2.ListDomains(request)
}

func (m *metadataManagerProxy) GetMetadata() (*p.GetMetadataResponse, error) {
	return m.metadataMgrV2.GetMetadata()
}

func (m *metadataManagerProxy) Close() {
	m.metadataMgr.Close()
	m.metadataMgrV2.Close()
}

func (m *metadataManagerProxy) CreateDomain(request *p.CreateDomainRequest) (*p.CreateDomainResponse, error) {
	if request.IsGlobalDomain {
		return m.metadataMgrV2.CreateDomain(request)
	}

	return m.metadataMgr.CreateDomain(request)
}

func (m *metadataManagerProxy) UpdateDomain(request *p.UpdateDomainRequest) error {
	switch request.TableVersion {
	case p.DomainTableVersionV1:
		return m.metadataMgr.UpdateDomain(request)
	case p.DomainTableVersionV2:
		return m.metadataMgrV2.UpdateDomain(request)
	default:
		return errors.New("domain table version is not set")
	}
}

func (m *metadataManagerProxy) DeleteDomain(request *p.DeleteDomainRequest) error {
	err := m.metadataMgr.DeleteDomain(request)
	if err != nil {
		m.logger.Warn("Error deleting domain from V1 table", tag.Error(err))
	}
	err = m.metadataMgrV2.DeleteDomain(request)
	if err != nil {
		m.logger.Warn("Error deleting domain from V2 table", tag.Error(err))
	}
	return nil
}

func (m *metadataManagerProxy) DeleteDomainByName(request *p.DeleteDomainByNameRequest) error {
	err := m.metadataMgr.DeleteDomainByName(request)
	if err != nil {
		m.logger.Warn("Error deleting domain by name from V1 table", tag.Error(err))
	}
	err = m.metadataMgrV2.DeleteDomainByName(request)
	if err != nil {
		m.logger.Warn("Error deleting domain by name from V2 table", tag.Error(err))
	}
	return nil
}
