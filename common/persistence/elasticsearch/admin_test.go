package elasticsearch

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/elasticsearch/client"
	"github.com/uber/cadence/common/persistence"
)

func TestAdminDB_MetadataAndSchema(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		wantPlugin string
	}{
		{name: "elasticsearch plugin for v6", version: v6, wantPlugin: "elasticsearch"},
		{name: "elasticsearch plugin for v7", version: v7, wantPlugin: "elasticsearch"},
		{name: "opensearch plugin for os2", version: os2, wantPlugin: "opensearch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testESConfig(t, tt.version, "cadence-visibility-test")
			a := &admin{cfg: cfg}

			assert.Equal(t, tt.wantPlugin, a.PluginName())
			assert.Equal(t, persistence.DBTypeVisibility, a.DBType())
			assert.Equal(t, cfg.URL.String()+"/"+cfg.GetVisibilityIndex(), a.Identifier())
			assert.False(t, a.SupportsSchema())

			schemaDB, err := a.CreateSchemaDB()
			require.ErrorContains(t, err, "not supported")
			assert.Nil(t, schemaDB)
		})
	}
}

func TestAdminDB_CreateSetupDB(t *testing.T) {
	tests := []struct {
		name            string
		indexName       string
		healthErr       error
		wantErrContains string
		wantSetup       bool
	}{
		{
			name:            "invalid index prefix",
			indexName:       "test-visibility",
			wantErrContains: "invalid visibility index",
			wantSetup:       false,
		},
		{
			name:            "health check fails",
			indexName:       "cadence-visibility-test",
			healthErr:       errors.New("ping failed"),
			wantErrContains: "ping failed",
			wantSetup:       false,
		},
		{
			name:      "success",
			indexName: "cadence-visibility-test",
			wantSetup: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockClient := client.NewMockClient(ctrl)
			cfg := testESConfig(t, v6, tt.indexName)

			a := &admin{client: mockClient, cfg: cfg}
			if tt.wantErrContains == "invalid visibility index" {
				// no calls expected
			} else {
				mockClient.EXPECT().IsHealthy(gomock.Any()).Return(tt.healthErr)
			}

			setupConn, err := a.CreateSetupDB()
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				assert.Nil(t, setupConn)
				return
			}

			require.NoError(t, err)
			if tt.wantSetup {
				require.NotNil(t, setupConn)
				s, ok := setupConn.(*setupDB)
				require.True(t, ok)
				assert.Equal(t, cfg.GetVisibilityIndex(), s.indexName)
				assert.Equal(t, visibilityTemplateName, s.templateName)
				assert.Same(t, mockClient, s.client)
			}
		})
	}
}

func TestSetupDB_IsSetup(t *testing.T) {
	latestTemplate := []byte("latest-template")
	equalMappings := map[string]any{
		"properties": map[string]any{
			"WorkflowID": map[string]any{"type": "keyword"},
		},
	}
	differentMappings := map[string]any{
		"properties": map[string]any{},
	}

	tests := []struct {
		name            string
		allowances      func(*setupDB, *client.MockClient)
		wantSetup       bool
		wantErrContains string
	}{
		{
			name: "has index error",
			allowances: func(s *setupDB, client *client.MockClient) {
				client.EXPECT().HasIndex(gomock.Any(), s.indexName).Return(false, errors.New("exists failed"))
			},
			wantErrContains: "exists failed",
		},
		{
			name: "index does not exist",
			allowances: func(s *setupDB, client *client.MockClient) {
				client.EXPECT().HasIndex(gomock.Any(), s.indexName).Return(false, nil)
			},
			wantSetup: false,
		},
		{
			name: "get mappings error",
			allowances: func(s *setupDB, client *client.MockClient) {
				client.EXPECT().HasIndex(gomock.Any(), s.indexName).Return(true, nil)
				client.EXPECT().GetMappings(gomock.Any(), s.indexName).Return(nil, errors.New("get mappings failed"))
			},
			wantErrContains: "get mappings failed",
		},
		{
			name: "latest template mapping parse error",
			allowances: func(s *setupDB, client *client.MockClient) {
				client.EXPECT().HasIndex(gomock.Any(), s.indexName).Return(true, nil)
				client.EXPECT().GetMappings(gomock.Any(), s.indexName).Return(equalMappings, nil)
				client.EXPECT().LatestTemplate().Return(latestTemplate)
				client.EXPECT().MappingsFromTemplate(latestTemplate).Return(nil, errors.New("parse template failed"))
			},
			wantErrContains: "parse template failed",
		},
		{
			name: "equal mappings returns true",
			allowances: func(s *setupDB, client *client.MockClient) {
				client.EXPECT().HasIndex(gomock.Any(), s.indexName).Return(true, nil)
				client.EXPECT().GetMappings(gomock.Any(), s.indexName).Return(equalMappings, nil)
				client.EXPECT().LatestTemplate().Return(latestTemplate)
				client.EXPECT().MappingsFromTemplate(latestTemplate).Return(equalMappings, nil)
			},
			wantSetup: true,
		},
		{
			name: "different mappings returns false",
			allowances: func(s *setupDB, client *client.MockClient) {
				client.EXPECT().HasIndex(gomock.Any(), s.indexName).Return(true, nil)
				client.EXPECT().GetMappings(gomock.Any(), s.indexName).Return(differentMappings, nil)
				client.EXPECT().LatestTemplate().Return(latestTemplate)
				client.EXPECT().MappingsFromTemplate(latestTemplate).Return(equalMappings, nil)
			},
			wantSetup: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockClient := client.NewMockClient(ctrl)
			s := &setupDB{client: mockClient, indexName: "cadence-visibility-test", templateName: visibilityTemplateName}
			tt.allowances(s, mockClient)

			got, err := s.IsSetup(t.Context())
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantSetup, got)
		})
	}
}

func TestSetupDB_Setup(t *testing.T) {
	latestTemplate := []byte("latest-template")
	latestMappings := map[string]any{"properties": map[string]any{"WorkflowID": map[string]any{"type": "keyword"}}}

	tests := []struct {
		name            string
		allowances      func(*setupDB, *client.MockClient)
		wantErrContains string
	}{
		{
			name: "latest mappings parse error",
			allowances: func(s *setupDB, client *client.MockClient) {
				client.EXPECT().LatestTemplate().Return(latestTemplate)
				client.EXPECT().MappingsFromTemplate(latestTemplate).Return(nil, errors.New("parse failed"))
			},
			wantErrContains: "read latest mappings",
		},
		{
			name: "has index error",
			allowances: func(s *setupDB, client *client.MockClient) {
				client.EXPECT().LatestTemplate().Return(latestTemplate)
				client.EXPECT().MappingsFromTemplate(latestTemplate).Return(latestMappings, nil)
				client.EXPECT().HasIndex(gomock.Any(), s.indexName).Return(false, errors.New("exists failed"))
			},
			wantErrContains: "checking if index",
		},
		{
			name: "missing index put template fails",
			allowances: func(s *setupDB, client *client.MockClient) {
				client.EXPECT().LatestTemplate().Return(latestTemplate)
				client.EXPECT().MappingsFromTemplate(latestTemplate).Return(latestMappings, nil)
				client.EXPECT().HasIndex(gomock.Any(), s.indexName).Return(false, nil)
				client.EXPECT().PutIndexTemplate(gomock.Any(), s.templateName, latestTemplate).Return(errors.New("template failed"))
			},
			wantErrContains: "put index template",
		},
		{
			name: "missing index create fails",
			allowances: func(s *setupDB, client *client.MockClient) {
				client.EXPECT().LatestTemplate().Return(latestTemplate)
				client.EXPECT().MappingsFromTemplate(latestTemplate).Return(latestMappings, nil)
				client.EXPECT().HasIndex(gomock.Any(), s.indexName).Return(false, nil)
				client.EXPECT().PutIndexTemplate(gomock.Any(), s.templateName, latestTemplate).Return(nil)
				client.EXPECT().CreateIndex(gomock.Any(), s.indexName).Return(errors.New("create failed"))
			},
			wantErrContains: "create index",
		},
		{
			name: "put mappings fails after create",
			allowances: func(s *setupDB, client *client.MockClient) {
				client.EXPECT().LatestTemplate().Return(latestTemplate)
				client.EXPECT().MappingsFromTemplate(latestTemplate).Return(latestMappings, nil)
				client.EXPECT().HasIndex(gomock.Any(), s.indexName).Return(false, nil)
				client.EXPECT().PutIndexTemplate(gomock.Any(), s.templateName, latestTemplate).Return(nil)
				client.EXPECT().CreateIndex(gomock.Any(), s.indexName).Return(nil)
				client.EXPECT().PutMappings(gomock.Any(), s.indexName, latestMappings).Return(errors.New("put mappings failed"))
			},
			wantErrContains: "put mappings for",
		},
		{
			name: "put mappings fails for existing index",
			allowances: func(s *setupDB, client *client.MockClient) {
				client.EXPECT().LatestTemplate().Return(latestTemplate)
				client.EXPECT().MappingsFromTemplate(latestTemplate).Return(latestMappings, nil)
				client.EXPECT().HasIndex(gomock.Any(), s.indexName).Return(true, nil)
				client.EXPECT().PutMappings(gomock.Any(), s.indexName, latestMappings).Return(errors.New("put mappings failed"))
			},
			wantErrContains: "put mappings for",
		},
		{
			name: "success existing index",
			allowances: func(s *setupDB, client *client.MockClient) {
				client.EXPECT().LatestTemplate().Return(latestTemplate)
				client.EXPECT().MappingsFromTemplate(latestTemplate).Return(latestMappings, nil)
				client.EXPECT().HasIndex(gomock.Any(), s.indexName).Return(true, nil)
				client.EXPECT().PutMappings(gomock.Any(), s.indexName, latestMappings).Return(nil)
			},
		},
		{
			name: "success new index",
			allowances: func(s *setupDB, client *client.MockClient) {
				client.EXPECT().LatestTemplate().Return(latestTemplate)
				client.EXPECT().MappingsFromTemplate(latestTemplate).Return(latestMappings, nil)
				client.EXPECT().HasIndex(gomock.Any(), s.indexName).Return(false, nil)
				client.EXPECT().PutIndexTemplate(gomock.Any(), s.templateName, latestTemplate).Return(nil)
				client.EXPECT().CreateIndex(gomock.Any(), s.indexName).Return(nil)
				client.EXPECT().PutMappings(gomock.Any(), s.indexName, latestMappings).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockClient := client.NewMockClient(ctrl)
			s := &setupDB{client: mockClient, indexName: "cadence-visibility-test", templateName: visibilityTemplateName}
			tt.allowances(s, mockClient)

			err := s.Setup(context.Background(), nil)
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestSetupDB_Teardown(t *testing.T) {
	tests := []struct {
		name            string
		hasIndex        bool
		hasIndexErr     error
		deleteErr       error
		wantErrContains string
	}{
		{
			name:            "has index error",
			hasIndexErr:     errors.New("exists failed"),
			wantErrContains: "exists failed",
		},
		{
			name:     "index missing no-op",
			hasIndex: false,
		},
		{
			name:            "delete index error",
			hasIndex:        true,
			deleteErr:       errors.New("delete failed"),
			wantErrContains: "delete failed",
		},
		{
			name:     "delete index success",
			hasIndex: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockClient := client.NewMockClient(ctrl)
			s := &setupDB{client: mockClient, indexName: "cadence-visibility-test", templateName: visibilityTemplateName}

			mockClient.EXPECT().HasIndex(gomock.Any(), s.indexName).Return(tt.hasIndex, tt.hasIndexErr)
			if tt.hasIndexErr == nil && tt.hasIndex {
				mockClient.EXPECT().DeleteIndex(gomock.Any(), s.indexName).Return(tt.deleteErr)
			}

			err := s.Teardown(context.Background())
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestSetupDB_Close(t *testing.T) {
	s := &setupDB{}
	s.Close()
}

func testESConfig(t *testing.T, version, visibilityIndex string) *config.ElasticSearchConfig {
	t.Helper()
	parsedURL, err := url.Parse("http://127.0.0.1:9200")
	require.NoError(t, err)
	return &config.ElasticSearchConfig{
		Version: version,
		URL:     *parsedURL,
		Indices: map[string]string{"visibility": visibilityIndex},
	}
}
