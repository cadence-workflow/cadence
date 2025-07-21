// Copyright (c) 2018 Uber Technologies, Inc.
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

package sql

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/types"
)

type sqlResult struct {
	lastInsertID int64
	rowsAffected int64
	err          error
}

func (r *sqlResult) LastInsertId() (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.lastInsertID, nil
}

func (r *sqlResult) RowsAffected() (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.rowsAffected, nil
}

func TestGetMetadata(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*sqlplugin.MockDB)
		want      *persistence.GetMetadataResponse
		wantErr   bool
	}{
		{
			name: "Success case",
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().SelectFromDomainMetadata(gomock.Any()).Return(&sqlplugin.DomainMetadataRow{NotificationVersion: 2}, nil)
			},
			want:    &persistence.GetMetadataResponse{NotificationVersion: 2},
			wantErr: false,
		},
		{
			name: "Error case",
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("some error")
				mockDB.EXPECT().SelectFromDomainMetadata(gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store := &sqlDomainStore{
				sqlStore: sqlStore{db: mockDB},
			}

			tc.mockSetup(mockDB)

			got, err := store.GetMetadata(context.Background())
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestListDomains(t *testing.T) {
	testCases := []struct {
		name              string
		activeClusterName string
		req               *persistence.ListDomainsRequest
		mockSetup         func(*sqlplugin.MockDB, *serialization.MockParser)
		want              *persistence.InternalListDomainsResponse
		wantErr           bool
	}{
		{
			name:              "Success case",
			activeClusterName: "active",
			req: &persistence.ListDomainsRequest{
				NextPageToken: []byte(`7a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c`),
				PageSize:      1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				uuid := serialization.UUID(`7a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c`)
				mockDB.EXPECT().SelectFromDomain(gomock.Any(), &sqlplugin.DomainFilter{
					GreaterThanID: &uuid,
					PageSize:      common.IntPtr(1),
				}).Return([]sqlplugin.DomainRow{
					{
						ID:           serialization.MustParseUUID("9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c"),
						Name:         "test",
						Data:         []byte(`aaaa`),
						DataEncoding: string(constants.EncodingTypeThriftRW),
					},
				}, nil)
				mockParser.EXPECT().DomainInfoFromBlob([]byte(`aaaa`), string(constants.EncodingTypeThriftRW)).
					Return(&serialization.DomainInfo{
						ActiveClustersConfig:         []byte(`active-clusters-config`),
						ActiveClustersConfigEncoding: string(constants.EncodingTypeThriftRW),
					}, nil)
			},
			want: &persistence.InternalListDomainsResponse{
				Domains: []*persistence.InternalGetDomainResponse{
					{
						Info: &persistence.DomainInfo{
							ID:   "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
							Name: "test",
						},
						Config: &persistence.InternalDomainConfig{},
						ReplicationConfig: &persistence.InternalDomainReplicationConfig{
							ActiveClusterName: "active",
							Clusters: []*persistence.ClusterReplicationConfig{
								{ClusterName: "active"},
							},
							ActiveClustersConfig: &persistence.DataBlob{
								Data:     []byte(`active-clusters-config`),
								Encoding: constants.EncodingTypeThriftRW,
							},
						},
					},
				},
				NextPageToken: serialization.MustParseUUID("9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c"),
			},
			wantErr: false,
		},
		{
			name:              "No Record case",
			activeClusterName: "active",
			req: &persistence.ListDomainsRequest{
				NextPageToken: []byte(`7a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c`),
				PageSize:      1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				uuid := serialization.UUID(`7a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c`)
				mockDB.EXPECT().SelectFromDomain(gomock.Any(), &sqlplugin.DomainFilter{
					GreaterThanID: &uuid,
					PageSize:      common.IntPtr(1),
				}).Return(nil, sql.ErrNoRows)
			},
			want:    &persistence.InternalListDomainsResponse{},
			wantErr: false,
		},
		{
			name:              "Error from database case",
			activeClusterName: "active",
			req: &persistence.ListDomainsRequest{
				NextPageToken: []byte(`7a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c`),
				PageSize:      1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				uuid := serialization.UUID(`7a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c`)
				err := errors.New("some error")
				mockDB.EXPECT().SelectFromDomain(gomock.Any(), &sqlplugin.DomainFilter{
					GreaterThanID: &uuid,
					PageSize:      common.IntPtr(1),
				}).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:              "Error from corrupted data case",
			activeClusterName: "active",
			req: &persistence.ListDomainsRequest{
				NextPageToken: []byte(`7a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c`),
				PageSize:      1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				uuid := serialization.UUID(`7a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c`)
				mockDB.EXPECT().SelectFromDomain(gomock.Any(), &sqlplugin.DomainFilter{
					GreaterThanID: &uuid,
					PageSize:      common.IntPtr(1),
				}).Return([]sqlplugin.DomainRow{
					{
						ID:           serialization.MustParseUUID("9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c"),
						Name:         "test",
						Data:         []byte(`aaaa`),
						DataEncoding: string(constants.EncodingTypeThriftRW),
					},
				}, nil)
				mockParser.EXPECT().DomainInfoFromBlob([]byte(`aaaa`), string(constants.EncodingTypeThriftRW)).Return(nil, errors.New("corrupted blob"))
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			mockParser := serialization.NewMockParser(ctrl)
			store := &sqlDomainStore{
				sqlStore:          sqlStore{db: mockDB, parser: mockParser},
				activeClusterName: tc.activeClusterName,
			}

			tc.mockSetup(mockDB, mockParser)

			got, err := store.ListDomains(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestGetDomain(t *testing.T) {
	testCases := []struct {
		name              string
		activeClusterName string
		req               *persistence.GetDomainRequest
		mockSetup         func(*sqlplugin.MockDB, *serialization.MockParser)
		want              *persistence.InternalGetDomainResponse
		wantErr           bool
		assertErr         func(*testing.T, error)
	}{
		{
			name:              "Success case - get domain by name",
			activeClusterName: "active",
			req: &persistence.GetDomainRequest{
				Name: "test",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromDomain(gomock.Any(), &sqlplugin.DomainFilter{
					Name: common.StringPtr("test"),
				}).Return([]sqlplugin.DomainRow{
					{
						ID:           serialization.MustParseUUID("9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c"),
						Name:         "test",
						Data:         []byte(`aaaa`),
						DataEncoding: string(constants.EncodingTypeThriftRW),
					},
				}, nil)
				mockParser.EXPECT().DomainInfoFromBlob([]byte(`aaaa`), string(constants.EncodingTypeThriftRW)).
					Return(&serialization.DomainInfo{
						ActiveClustersConfig:         []byte(`active-clusters-config`),
						ActiveClustersConfigEncoding: string(constants.EncodingTypeThriftRW),
					}, nil)
			},
			want: &persistence.InternalGetDomainResponse{
				Info: &persistence.DomainInfo{
					ID:   "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
					Name: "test",
				},
				Config: &persistence.InternalDomainConfig{},
				ReplicationConfig: &persistence.InternalDomainReplicationConfig{
					ActiveClusterName: "active",
					Clusters: []*persistence.ClusterReplicationConfig{
						{
							ClusterName: "active",
						},
					},
					ActiveClustersConfig: &persistence.DataBlob{
						Data:     []byte(`active-clusters-config`),
						Encoding: constants.EncodingTypeThriftRW,
					},
				},
			},
			wantErr: false,
		},
		{
			name:              "Success case - get domain by ID",
			activeClusterName: "active",
			req: &persistence.GetDomainRequest{
				ID: "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				uuid := serialization.MustParseUUID("9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c")
				mockDB.EXPECT().SelectFromDomain(gomock.Any(), &sqlplugin.DomainFilter{
					ID: &uuid,
				}).Return([]sqlplugin.DomainRow{
					{
						ID:           serialization.MustParseUUID("9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c"),
						Name:         "test",
						Data:         []byte(`aaaa`),
						DataEncoding: string(constants.EncodingTypeThriftRW),
					},
				}, nil)
				mockParser.EXPECT().DomainInfoFromBlob([]byte(`aaaa`), string(constants.EncodingTypeThriftRW)).Return(&serialization.DomainInfo{}, nil)
			},
			want: &persistence.InternalGetDomainResponse{
				Info: &persistence.DomainInfo{
					ID:   "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
					Name: "test",
				},
				Config: &persistence.InternalDomainConfig{},
				ReplicationConfig: &persistence.InternalDomainReplicationConfig{
					ActiveClusterName: "active",
					Clusters: []*persistence.ClusterReplicationConfig{
						{
							ClusterName: "active",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:              "Error case - both domain name and ID are set in request",
			activeClusterName: "active",
			req: &persistence.GetDomainRequest{
				Name: "test",
				ID:   "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {},
			wantErr:   true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *types.BadRequestError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be BadRequestError")
				assert.Contains(t, err.Error(), "GetDomain operation failed.  Both ID and Name specified in request.")
			},
		},
		{
			name:              "Error case - both domain name and ID are NOT set in request",
			activeClusterName: "active",
			req:               &persistence.GetDomainRequest{},
			mockSetup:         func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {},
			wantErr:           true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *types.BadRequestError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be BadRequestError")
				assert.Contains(t, err.Error(), "GetDomain operation failed.  Both ID and Name are empty.")
			},
		},
		{
			name:              "No Record case",
			activeClusterName: "active",
			req: &persistence.GetDomainRequest{
				Name: "test",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromDomain(gomock.Any(), &sqlplugin.DomainFilter{
					Name: common.StringPtr("test"),
				}).Return(nil, sql.ErrNoRows)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *types.EntityNotExistsError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be EntityNotExistsError")
			},
		},
		{
			name:              "Error from database case",
			activeClusterName: "active",
			req: &persistence.GetDomainRequest{
				Name: "test",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				err := errors.New("some error")
				mockDB.EXPECT().SelectFromDomain(gomock.Any(), &sqlplugin.DomainFilter{
					Name: common.StringPtr("test"),
				}).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:              "Error from corrupted data case",
			activeClusterName: "active",
			req: &persistence.GetDomainRequest{
				Name: "test",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromDomain(gomock.Any(), &sqlplugin.DomainFilter{
					Name: common.StringPtr("test"),
				}).Return([]sqlplugin.DomainRow{
					{
						ID:           serialization.MustParseUUID("9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c"),
						Name:         "test",
						Data:         []byte(`aaaa`),
						DataEncoding: string(constants.EncodingTypeThriftRW),
					},
				}, nil)
				mockParser.EXPECT().DomainInfoFromBlob([]byte(`aaaa`), string(constants.EncodingTypeThriftRW)).Return(nil, errors.New("corrupted blob"))
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			mockParser := serialization.NewMockParser(ctrl)
			store := &sqlDomainStore{
				sqlStore:          sqlStore{db: mockDB, parser: mockParser},
				activeClusterName: tc.activeClusterName,
			}

			tc.mockSetup(mockDB, mockParser)

			got, err := store.GetDomain(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestCreateDomain(t *testing.T) {
	testCases := []struct {
		name              string
		activeClusterName string
		req               *persistence.InternalCreateDomainRequest
		mockSetup         func(*sqlplugin.MockDB, *sqlplugin.MockTx, *serialization.MockParser)
		want              *persistence.CreateDomainResponse
		wantErr           bool
		assertErr         func(*testing.T, error)
	}{
		{
			name:              "Success case",
			activeClusterName: "active",
			req: &persistence.InternalCreateDomainRequest{
				Info: &persistence.DomainInfo{
					ID:          "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
					Name:        "test",
					Status:      1,
					Description: "n/a",
					OwnerEmail:  "abc@xyz.com",
					Data:        map[string]string{"k": "v"},
				},
				Config: &persistence.InternalDomainConfig{
					Retention:                time.Hour * 24,
					EmitMetric:               true,
					HistoryArchivalStatus:    types.ArchivalStatusEnabled,
					HistoryArchivalURI:       "http://a.b",
					VisibilityArchivalStatus: types.ArchivalStatusEnabled,
					VisibilityArchivalURI:    "http://x.y",
					BadBinaries:              nil,
					IsolationGroups:          nil,
				},
				ReplicationConfig: &persistence.InternalDomainReplicationConfig{
					ActiveClusterName: "active",
					Clusters: []*persistence.ClusterReplicationConfig{
						{
							ClusterName: "active",
						},
					},
					ActiveClustersConfig: &persistence.DataBlob{
						Data:     []byte(`active-clusters-config`),
						Encoding: constants.EncodingTypeThriftRW,
					},
				},
				ConfigVersion:   1,
				FailoverVersion: 3,
				IsGlobalDomain:  true,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromDomainMetadata(gomock.Any()).Return(&sqlplugin.DomainMetadataRow{NotificationVersion: 2}, nil)
				mockParser.EXPECT().DomainInfoToBlob(&serialization.DomainInfo{
					Name:                         "test",
					Status:                       1,
					Description:                  "n/a",
					Owner:                        "abc@xyz.com",
					Data:                         map[string]string{"k": "v"},
					Retention:                    time.Hour * 24,
					EmitMetric:                   true,
					HistoryArchivalStatus:        int16(types.ArchivalStatusEnabled),
					HistoryArchivalURI:           "http://a.b",
					VisibilityArchivalStatus:     int16(types.ArchivalStatusEnabled),
					VisibilityArchivalURI:        "http://x.y",
					ActiveClusterName:            "active",
					Clusters:                     []string{"active"},
					ConfigVersion:                1,
					FailoverVersion:              3,
					NotificationVersion:          2,
					FailoverNotificationVersion:  persistence.InitialFailoverNotificationVersion,
					PreviousFailoverVersion:      constants.InitialPreviousFailoverVersion,
					ActiveClustersConfig:         []byte(`active-clusters-config`),
					ActiveClustersConfigEncoding: string(constants.EncodingTypeThriftRW),
				}).Return(persistence.DataBlob{Data: []byte(`aaaa`), Encoding: constants.EncodingTypeThriftRW}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().InsertIntoDomain(gomock.Any(), &sqlplugin.DomainRow{
					Name:         "test",
					ID:           serialization.MustParseUUID("9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c"),
					Data:         []byte(`aaaa`),
					DataEncoding: string(constants.EncodingTypeThriftRW),
					IsGlobal:     true,
				}).Return(nil, nil)
				mockTx.EXPECT().LockDomainMetadata(gomock.Any()).Return(nil)
				mockTx.EXPECT().UpdateDomainMetadata(gomock.Any(), &sqlplugin.DomainMetadataRow{NotificationVersion: 2}).Return(&sqlResult{rowsAffected: 1}, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			want:    &persistence.CreateDomainResponse{ID: "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c"},
			wantErr: false,
		},
		{
			name:              "Error case - unable to get metadata",
			activeClusterName: "active",
			req:               &persistence.InternalCreateDomainRequest{},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				err := errors.New("some error")
				mockDB.EXPECT().SelectFromDomainMetadata(gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:              "Error case - unable to encode data",
			activeClusterName: "active",
			req: &persistence.InternalCreateDomainRequest{
				Info: &persistence.DomainInfo{
					ID:   "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
					Name: "test",
				},
				Config:            &persistence.InternalDomainConfig{},
				ReplicationConfig: &persistence.InternalDomainReplicationConfig{},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromDomainMetadata(gomock.Any()).Return(&sqlplugin.DomainMetadataRow{NotificationVersion: 2}, nil)
				mockParser.EXPECT().DomainInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name:              "Error case - unable to insert row",
			activeClusterName: "active",
			req: &persistence.InternalCreateDomainRequest{
				Info: &persistence.DomainInfo{
					ID:   "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
					Name: "test",
				},
				Config:            &persistence.InternalDomainConfig{},
				ReplicationConfig: &persistence.InternalDomainReplicationConfig{},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromDomainMetadata(gomock.Any()).Return(&sqlplugin.DomainMetadataRow{NotificationVersion: 2}, nil)
				mockParser.EXPECT().DomainInfoToBlob(gomock.Any()).Return(persistence.DataBlob{Data: []byte(`aaaa`), Encoding: constants.EncodingTypeThriftRW}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				err := errors.New("some error")
				mockTx.EXPECT().InsertIntoDomain(gomock.Any(), gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsDupEntryError(err).Return(false)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:              "Error case - domain already exists",
			activeClusterName: "active",
			req: &persistence.InternalCreateDomainRequest{
				Info: &persistence.DomainInfo{
					ID:   "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
					Name: "test",
				},
				Config:            &persistence.InternalDomainConfig{},
				ReplicationConfig: &persistence.InternalDomainReplicationConfig{},
				ConfigVersion:     1,
				FailoverVersion:   3,
				IsGlobalDomain:    true,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromDomainMetadata(gomock.Any()).Return(&sqlplugin.DomainMetadataRow{NotificationVersion: 2}, nil)
				mockParser.EXPECT().DomainInfoToBlob(gomock.Any()).Return(persistence.DataBlob{Data: []byte(`aaaa`), Encoding: constants.EncodingTypeThriftRW}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				err := errors.New("some error")
				mockTx.EXPECT().InsertIntoDomain(gomock.Any(), gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsDupEntryError(err).Return(true)
				mockTx.EXPECT().Rollback().Return(nil)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *types.DomainAlreadyExistsError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be DomainAlreadyExistsError")
			},
		},
		{
			name:              "Error case - unable to lock metadata",
			activeClusterName: "active",
			req: &persistence.InternalCreateDomainRequest{
				Info: &persistence.DomainInfo{
					ID:   "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
					Name: "test",
				},
				Config:            &persistence.InternalDomainConfig{},
				ReplicationConfig: &persistence.InternalDomainReplicationConfig{},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromDomainMetadata(gomock.Any()).Return(&sqlplugin.DomainMetadataRow{NotificationVersion: 2}, nil)
				mockParser.EXPECT().DomainInfoToBlob(gomock.Any()).Return(persistence.DataBlob{Data: []byte(`aaaa`), Encoding: constants.EncodingTypeThriftRW}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().InsertIntoDomain(gomock.Any(), gomock.Any()).Return(nil, nil)
				err := errors.New("some error")
				mockTx.EXPECT().LockDomainMetadata(gomock.Any()).Return(err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
				mockTx.EXPECT().Rollback().Return(nil)
			},
			wantErr: true,
		},
		{
			name:              "Error case - unable to update metadata",
			activeClusterName: "active",
			req: &persistence.InternalCreateDomainRequest{
				Info: &persistence.DomainInfo{
					ID:   "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
					Name: "test",
				},
				Config:            &persistence.InternalDomainConfig{},
				ReplicationConfig: &persistence.InternalDomainReplicationConfig{},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromDomainMetadata(gomock.Any()).Return(&sqlplugin.DomainMetadataRow{NotificationVersion: 2}, nil)
				mockParser.EXPECT().DomainInfoToBlob(gomock.Any()).Return(persistence.DataBlob{Data: []byte(`aaaa`), Encoding: constants.EncodingTypeThriftRW}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().InsertIntoDomain(gomock.Any(), gomock.Any()).Return(nil, nil)
				mockTx.EXPECT().LockDomainMetadata(gomock.Any()).Return(nil)
				err := errors.New("some error")
				mockTx.EXPECT().UpdateDomainMetadata(gomock.Any(), &sqlplugin.DomainMetadataRow{NotificationVersion: 2}).Return(nil, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
				mockTx.EXPECT().Rollback().Return(nil)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			mockTx := sqlplugin.NewMockTx(ctrl)
			mockParser := serialization.NewMockParser(ctrl)
			store := &sqlDomainStore{
				sqlStore:          sqlStore{db: mockDB, parser: mockParser},
				activeClusterName: tc.activeClusterName,
			}

			tc.mockSetup(mockDB, mockTx, mockParser)
			got, err := store.CreateDomain(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestUpdateDomain(t *testing.T) {
	testCases := []struct {
		name              string
		activeClusterName string
		req               *persistence.InternalUpdateDomainRequest
		mockSetup         func(*sqlplugin.MockDB, *sqlplugin.MockTx, *serialization.MockParser)
		wantErr           bool
		assertErr         func(*testing.T, error)
	}{
		{
			name:              "Success case",
			activeClusterName: "active",
			req: &persistence.InternalUpdateDomainRequest{
				Info: &persistence.DomainInfo{
					ID:          "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
					Name:        "test",
					Status:      1,
					Description: "n/a",
					OwnerEmail:  "abc@xyz.com",
					Data:        map[string]string{"k": "v"},
				},
				Config: &persistence.InternalDomainConfig{
					Retention:                time.Hour * 24,
					EmitMetric:               true,
					HistoryArchivalStatus:    types.ArchivalStatusEnabled,
					HistoryArchivalURI:       "http://a.b",
					VisibilityArchivalStatus: types.ArchivalStatusEnabled,
					VisibilityArchivalURI:    "http://x.y",
					BadBinaries:              nil,
					IsolationGroups:          nil,
				},
				ReplicationConfig: &persistence.InternalDomainReplicationConfig{
					ActiveClusterName: "active",
					Clusters: []*persistence.ClusterReplicationConfig{
						{
							ClusterName: "active",
						},
					},
					ActiveClustersConfig: &persistence.DataBlob{
						Data:     []byte(`active-clusters-config`),
						Encoding: constants.EncodingTypeThriftRW,
					},
				},
				ConfigVersion:               1,
				FailoverVersion:             3,
				FailoverNotificationVersion: 4,
				PreviousFailoverVersion:     5,
				NotificationVersion:         2,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().DomainInfoToBlob(&serialization.DomainInfo{
					Status:                       1,
					Description:                  "n/a",
					Owner:                        "abc@xyz.com",
					Data:                         map[string]string{"k": "v"},
					Retention:                    time.Hour * 24,
					EmitMetric:                   true,
					HistoryArchivalStatus:        int16(types.ArchivalStatusEnabled),
					HistoryArchivalURI:           "http://a.b",
					VisibilityArchivalStatus:     int16(types.ArchivalStatusEnabled),
					VisibilityArchivalURI:        "http://x.y",
					ActiveClusterName:            "active",
					Clusters:                     []string{"active"},
					ConfigVersion:                1,
					FailoverVersion:              3,
					NotificationVersion:          2,
					FailoverNotificationVersion:  4,
					PreviousFailoverVersion:      5,
					ActiveClustersConfig:         []byte(`active-clusters-config`),
					ActiveClustersConfigEncoding: string(constants.EncodingTypeThriftRW),
				}).Return(persistence.DataBlob{Data: []byte(`aaaa`), Encoding: constants.EncodingTypeThriftRW}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().UpdateDomain(gomock.Any(), &sqlplugin.DomainRow{
					Name:         "test",
					ID:           serialization.MustParseUUID("9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c"),
					Data:         []byte(`aaaa`),
					DataEncoding: string(constants.EncodingTypeThriftRW),
				}).Return(&sqlResult{rowsAffected: 1}, nil)
				mockTx.EXPECT().LockDomainMetadata(gomock.Any()).Return(nil)
				mockTx.EXPECT().UpdateDomainMetadata(gomock.Any(), &sqlplugin.DomainMetadataRow{NotificationVersion: 2}).Return(&sqlResult{rowsAffected: 1}, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			wantErr: false,
		},
		{
			name:              "Error case - unable to encode data",
			activeClusterName: "active",
			req: &persistence.InternalUpdateDomainRequest{
				Info: &persistence.DomainInfo{
					ID:   "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
					Name: "test",
				},
				Config:            &persistence.InternalDomainConfig{},
				ReplicationConfig: &persistence.InternalDomainReplicationConfig{},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().DomainInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name:              "Error case - unable to update row",
			activeClusterName: "active",
			req: &persistence.InternalUpdateDomainRequest{
				Info: &persistence.DomainInfo{
					ID:   "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
					Name: "test",
				},
				Config:            &persistence.InternalDomainConfig{},
				ReplicationConfig: &persistence.InternalDomainReplicationConfig{},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().DomainInfoToBlob(gomock.Any()).Return(persistence.DataBlob{Data: []byte(`aaaa`), Encoding: constants.EncodingTypeThriftRW}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				err := errors.New("some error")
				mockTx.EXPECT().UpdateDomain(gomock.Any(), &sqlplugin.DomainRow{
					Name:         "test",
					ID:           serialization.MustParseUUID("9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c"),
					Data:         []byte(`aaaa`),
					DataEncoding: string(constants.EncodingTypeThriftRW),
				}).Return(nil, err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:              "Error case - unable to lock metadata",
			activeClusterName: "active",
			req: &persistence.InternalUpdateDomainRequest{
				Info: &persistence.DomainInfo{
					ID:   "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
					Name: "test",
				},
				Config:            &persistence.InternalDomainConfig{},
				ReplicationConfig: &persistence.InternalDomainReplicationConfig{},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().DomainInfoToBlob(gomock.Any()).Return(persistence.DataBlob{Data: []byte(`aaaa`), Encoding: constants.EncodingTypeThriftRW}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().UpdateDomain(gomock.Any(), &sqlplugin.DomainRow{
					Name:         "test",
					ID:           serialization.MustParseUUID("9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c"),
					Data:         []byte(`aaaa`),
					DataEncoding: string(constants.EncodingTypeThriftRW),
				}).Return(&sqlResult{rowsAffected: 1}, nil)
				err := errors.New("some error")
				mockTx.EXPECT().LockDomainMetadata(gomock.Any()).Return(err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
				mockTx.EXPECT().Rollback().Return(nil)
			},
			wantErr: true,
		},
		{
			name:              "Error case - unable to update metadata",
			activeClusterName: "active",
			req: &persistence.InternalUpdateDomainRequest{
				Info: &persistence.DomainInfo{
					ID:   "9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c",
					Name: "test",
				},
				Config:              &persistence.InternalDomainConfig{},
				ReplicationConfig:   &persistence.InternalDomainReplicationConfig{},
				NotificationVersion: 2,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().DomainInfoToBlob(gomock.Any()).Return(persistence.DataBlob{Data: []byte(`aaaa`), Encoding: constants.EncodingTypeThriftRW}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().UpdateDomain(gomock.Any(), &sqlplugin.DomainRow{
					Name:         "test",
					ID:           serialization.MustParseUUID("9a3dc7e2-1e67-41aa-8eaf-6d6e27f7e47c"),
					Data:         []byte(`aaaa`),
					DataEncoding: string(constants.EncodingTypeThriftRW),
				}).Return(&sqlResult{rowsAffected: 1}, nil)
				mockTx.EXPECT().LockDomainMetadata(gomock.Any()).Return(nil)
				err := errors.New("some error")
				mockTx.EXPECT().UpdateDomainMetadata(gomock.Any(), &sqlplugin.DomainMetadataRow{NotificationVersion: 2}).Return(nil, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
				mockTx.EXPECT().Rollback().Return(nil)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			mockTx := sqlplugin.NewMockTx(ctrl)
			mockParser := serialization.NewMockParser(ctrl)
			store := &sqlDomainStore{
				sqlStore:          sqlStore{db: mockDB, parser: mockParser},
				activeClusterName: tc.activeClusterName,
			}

			tc.mockSetup(mockDB, mockTx, mockParser)
			err := store.UpdateDomain(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}
