package elasticsearch_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windkube/aws-metrics-exporter/internal/config"
	"github.com/windkube/aws-metrics-exporter/internal/storage"
	"github.com/windkube/aws-metrics-exporter/internal/storage/elasticsearch"
)

func newClient(t *testing.T, handler http.HandlerFunc) *elasticsearch.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
	t.Cleanup(server.Close)

	client, err := elasticsearch.New(config.Elasticsearch{
		Addresses:   []string{server.URL},
		IndexPrefix: "aws-inventory",
		BatchSize:   500,
	})
	require.NoError(t, err)

	return client
}

func document() storage.Document {
	return storage.Document{
		ResourceType: "aws_iam_user",
		ResourceID:   "arn:aws:iam::111111111111:user/kw",
		ResourceName: "kw",
		AccountID:    "111111111111",
		AccountName:  "prod",
		Region:       "global",
		ScrapedAt:    time.Date(2026, 9, 18, 1, 0, 0, 0, time.UTC),
		Data:         json.RawMessage(`{"UserName":"kw"}`),
	}
}

func TestWriteIndexesIntoTheDailyIndexWithAStableID(t *testing.T) {
	var body string

	client := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		_, _ = w.Write([]byte(`{"errors":false,"items":[{"index":{"status":201}}]}`))
	})

	require.NoError(t, client.Write(t.Context(), []storage.Document{document()}))

	lines := strings.Split(strings.TrimSpace(body), "\n")
	require.Len(t, lines, 2)

	var meta struct {
		Index struct {
			Index string `json:"_index"`
			ID    string `json:"_id"`
		} `json:"index"`
	}
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &meta))
	assert.Equal(t, "aws-inventory-aws_iam_user-2026.09.18", meta.Index.Index)
	assert.Equal(t, "111111111111:global:aws_iam_user:arn:aws:iam::111111111111:user/kw", meta.Index.ID)

	var source map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &source))
	assert.Equal(t, "2026-09-18T01:00:00Z", source["@timestamp"])
	assert.Equal(t, map[string]any{"UserName": "kw"}, source["aws"])
	assert.Equal(t, "aws_iam_user", source["resource"].(map[string]any)["type"])
	assert.Equal(t, "prod", source["cloud"].(map[string]any)["account"].(map[string]any)["name"])
}

func TestWriteFailsWhenAnItemIsRejected(t *testing.T) {
	client := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"errors":true,"items":[
			{"index":{"_id":"a","status":201}},
			{"index":{"_id":"b","status":400,"error":{"type":"mapper_parsing_exception","reason":"bad field"}}}
		]}`))
	})

	err := client.Write(t.Context(), []storage.Document{document(), document()})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 of 2 documents rejected")
	assert.Contains(t, err.Error(), "mapper_parsing_exception")
}

func TestWriteIgnoresAnEmptyBatch(t *testing.T) {
	client := newClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("elasticsearch should not be called for an empty batch")
	})

	require.NoError(t, client.Write(t.Context(), nil))
}

func TestPingReportsAnUnhealthyCluster(t *testing.T) {
	client := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"unavailable"}`))
	})

	require.Error(t, client.Ping(context.Background()))
}
