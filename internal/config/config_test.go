package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windkube/aws-metrics-exporter/internal/config"
)

var known = []string{"aws_iam_user", "aws_elasticache_replication_group"}

func write(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	return path
}

const minimal = `
global:
  regions: [eu-west-1]
storage:
  elasticsearch:
    addresses: ["http://localhost:9200"]
resources: [aws_iam_user]
accounts:
  - name: prod
    id: "111111111111"
    role_arn: arn:aws:iam::111111111111:role/aws-metrics-exporter
`

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := config.Load(write(t, minimal), known)
	require.NoError(t, err)

	assert.Equal(t, time.Hour, cfg.Global.ScrapeInterval)
	assert.Equal(t, "info", cfg.Global.Log.Level)
	assert.Equal(t, ":8080", cfg.Global.Health.Address)
	assert.Equal(t, "aws-metrics-exporter", cfg.Global.Temporal.TaskQueue)
	assert.Equal(t, "elasticsearch", cfg.Storage.Backend)
	assert.Equal(t, "aws-inventory", cfg.Storage.Elasticsearch.IndexPrefix)
	assert.Equal(t, 500, cfg.Storage.Elasticsearch.BatchSize)
	assert.True(t, cfg.Storage.Elasticsearch.ManageIndexTemplate)
}

func TestLoadResolvesAccountDefaults(t *testing.T) {
	cfg, err := config.Load(write(t, minimal+`
  - name: sandbox
    id: "222222222222"
    role_arn: arn:aws:iam::222222222222:role/aws-metrics-exporter
    regions: [us-east-1, eu-central-1]
    scrape_interval: 6h
`), known)
	require.NoError(t, err)
	require.Len(t, cfg.Accounts, 2)

	prod, ok := cfg.Account("prod")
	require.True(t, ok)
	assert.Equal(t, []string{"eu-west-1"}, prod.Regions)
	assert.Equal(t, []string{"aws_iam_user"}, prod.Resources)
	assert.Equal(t, time.Hour, prod.ScrapeInterval)

	sandbox, ok := cfg.Account("sandbox")
	require.True(t, ok)
	assert.Equal(t, []string{"us-east-1", "eu-central-1"}, sandbox.Regions)
	assert.Equal(t, 6*time.Hour, sandbox.ScrapeInterval)
}

func TestLoadReadsCredentialsFromEnvironment(t *testing.T) {
	t.Setenv("AWSME_STORAGE_ELASTICSEARCH_USERNAME", "elastic")
	t.Setenv("AWSME_STORAGE_ELASTICSEARCH_PASSWORD", "hunter2")

	cfg, err := config.Load(write(t, minimal), known)
	require.NoError(t, err)

	assert.Equal(t, "elastic", cfg.Storage.Elasticsearch.Username)
	assert.Equal(t, "hunter2", cfg.Storage.Elasticsearch.Password)
}

func TestLoadRejectsUnknownResourceType(t *testing.T) {
	_, err := config.Load(write(t, `
storage:
  elasticsearch:
    addresses: ["http://localhost:9200"]
resources: [aws_unicorn]
accounts:
  - name: prod
    id: "111111111111"
    role_arn: arn:aws:iam::111111111111:role/aws-metrics-exporter
`), known)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown resource type "aws_unicorn"`)
}

func TestValidateReportsEveryProblemAtOnce(t *testing.T) {
	cfg := config.Config{
		Storage:  config.Storage{Backend: "clickhouse"},
		Accounts: []config.Account{{Name: "prod"}, {Name: "prod", ID: "1", RoleARN: "arn"}},
	}

	err := cfg.Validate(known)
	require.Error(t, err)

	for _, want := range []string{
		"storage.backend",
		"batch_size",
		"accounts.prod.id",
		"accounts.prod.role_arn",
		"duplicate account name",
		"scrape_interval",
	} {
		assert.Contains(t, err.Error(), want)
	}
}
