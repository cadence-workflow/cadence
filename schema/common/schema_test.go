package common

import (
	"embed"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed testdata/*
var TestDataFS embed.FS

var TestSchema = EmbeddedSchema(TestDataFS, "0.2", "testdata", "latest.sql")

func TestMetadata(t *testing.T) {
	updates, err := TestSchema.AllUpdates()
	require.NoError(t, err)
	// Assert that all other fields in the updates are set according to the corresponding manifest
	require.Len(t, updates, 2)

	// Check v0.1 metadata
	v01 := updates[0]
	require.Equal(t, "0.1", v01.Version.String())
	require.Equal(t, "0.1", v01.MinCompatibleVersion.String())
	require.Equal(t, "base version of schema", v01.Description)
	require.NotEmpty(t, v01.ManifestMD5)

	// Check v0.2 metadata
	v02 := updates[1]
	require.Equal(t, "0.2", v02.Version.String())
	require.Equal(t, "0.2", v02.MinCompatibleVersion.String())
	require.Equal(t, "cool new feature", v02.Description)
	require.NotEmpty(t, v02.ManifestMD5)

	// v0.1 and v0.2 should have different MD5s
	require.NotEqual(t, v01.ManifestMD5, v02.ManifestMD5)
}

func TestLoadLatest(t *testing.T) {
	update, err := TestSchema.SkipToLatest()
	require.NoError(t, err)
	statements := update.DDLStatements

	require.Len(t, statements, 3)
	require.Equal(t, "CREATE TABLE shards (shard_id INT NOT NULL,range_id BIGINT NOT NULL,data BLOB NOT NULL,data_encoding VARCHAR(16) NOT NULL,PRIMARY KEY (shard_id));", statements[0])
	require.Equal(t, "CREATE TABLE domain_metadata (`id` bigint(20) NOT NULL AUTO_INCREMENT,notification_version BIGINT NOT NULL,PRIMARY KEY (`id`));", statements[1])
	require.Equal(t, "INSERT INTO domain_metadata (notification_version) VALUES (1);", statements[2])
}

func TestLoadDDL(t *testing.T) {
	updates, err := TestSchema.AllUpdates()
	require.NoError(t, err)

	require.Len(t, updates[0].DDLStatements, 1)
	stmt := updates[0].DDLStatements[0]
	require.Equal(t, "CREATE TABLE shards (shard_id INT NOT NULL,range_id BIGINT NOT NULL,data BLOB NOT NULL,data_encoding VARCHAR(16) NOT NULL,PRIMARY KEY (shard_id));", stmt)
}

func TestLoadMultipleFiles(t *testing.T) {
	updates, err := TestSchema.AllUpdates()
	require.NoError(t, err)

	statements := updates[1].DDLStatements
	require.Len(t, statements, 3)
	// Ordering should match the order of the files in the manifest, and the order of each file
	require.Equal(t, "CREATE TABLE domain_metadata (`id` bigint(20) NOT NULL AUTO_INCREMENT,notification_version BIGINT NOT NULL,PRIMARY KEY (`id`));", statements[0])
	require.Equal(t, "INSERT INTO domain_metadata (notification_version) VALUES (1);", statements[1])
	require.Equal(t, "INSERT INTO domain_metadata (notification_version) VALUES (2);", statements[2])

}
