package elasticsearch

import (
	"context"
	"fmt"
	"strings"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/elasticsearch/client"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
)

const (
	visibilityTemplateName = "cadence-visibility-template"
	v6                     = "v6"
	v7                     = "v7"
	os2                    = "os2"
)

type admin struct {
	client client.Client
	cfg    *config.ElasticSearchConfig
}

func NewAdminDB(config *config.ElasticSearchConfig, logger log.Logger) (persistence.AdminDB, error) {
	client, err := elasticsearch.NewGenericClient(config, logger)
	if err != nil {
		return nil, err
	}
	return &admin{client: client.Client, cfg: config}, nil
}

func (a *admin) PluginName() string {
	if a.cfg.Version == os2 {
		return "opensearch"
	}
	return "elasticsearch"
}

func (a *admin) DBType() persistence.DBType {
	return persistence.DBTypeVisibility
}

func (a *admin) Identifier() string {
	return fmt.Sprintf("%s/%s", a.cfg.URL.String(), a.cfg.GetVisibilityIndex())
}

func (a *admin) CreateSetupDB() (persistence.SetupDB, error) {
	indexName := a.cfg.GetVisibilityIndex()
	if !strings.HasPrefix(indexName, "cadence-visibility-") {
		return nil, fmt.Errorf("invalid visibility index, must start with cadence-visibility- : %s", indexName)
	}

	err := a.client.IsHealthy(context.Background())
	if err != nil {
		return nil, err
	}

	return &setupDB{
		client:       a.client,
		indexName:    indexName,
		templateName: visibilityTemplateName,
	}, nil
}

func (a *admin) SupportsSchema() bool {
	return false
}

func (a *admin) CreateSchemaDB() (persistence.SchemaDB, error) {
	return nil, fmt.Errorf("not supported")
}

type setupDB struct {
	client       client.Client
	indexName    string
	templateName string
}

func (s *setupDB) IsSetup(ctx context.Context) (bool, error) {
	hasIndex, err := s.client.HasIndex(ctx, s.indexName)
	if err != nil {
		return false, fmt.Errorf("checking for index: %w", err)
	}
	if !hasIndex {
		return false, nil
	}
	currentMappings, err := s.client.GetMappings(ctx, s.indexName)
	if err != nil {
		return false, fmt.Errorf("getting mappings: %w", err)
	}
	latestMappings, err := s.client.MappingsFromTemplate(s.client.LatestTemplate())
	if err != nil {
		return false, fmt.Errorf("getting template mappings: %w", err)
	}

	missing, err := IsMissingMappings(currentMappings, latestMappings)

	return !missing, err
}

func (s *setupDB) Setup(ctx context.Context, _ map[string]string) error {
	latestTemplate := s.client.LatestTemplate()
	latestMappings, err := s.client.MappingsFromTemplate(latestTemplate)
	if err != nil {
		return fmt.Errorf("read latest mappings: %w", err)
	}

	isCreated, err := s.client.HasIndex(ctx, s.indexName)
	if err != nil {
		return fmt.Errorf("checking if index %q exists: %w", s.indexName, err)
	}
	if !isCreated {
		if err := s.client.PutIndexTemplate(ctx, s.templateName, latestTemplate); err != nil {
			return fmt.Errorf("put index template %q: %w", s.templateName, err)
		}
		if err := s.client.CreateIndex(ctx, s.indexName); err != nil {
			return fmt.Errorf("create index %q: %w", s.indexName, err)
		}
	}

	if err := s.client.PutMappings(ctx, s.indexName, latestMappings); err != nil {
		return fmt.Errorf("put mappings for %q: %w", s.indexName, err)
	}
	return nil
}

func (s *setupDB) Teardown(ctx context.Context) error {
	exists, err := s.client.HasIndex(ctx, s.indexName)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return s.client.DeleteIndex(ctx, s.indexName)
}

func (s *setupDB) Close() {}
