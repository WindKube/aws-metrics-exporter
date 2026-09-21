package elasticsearch

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/elastic/go-elasticsearch/v8"

	"github.com/windkube/aws-metrics-exporter/internal/config"
	"github.com/windkube/aws-metrics-exporter/internal/storage"
)

const indexDateLayout = "2006.01.02"

type Client struct {
	es          *elasticsearch.Client
	indexPrefix string
	batchSize   int
}

func New(cfg config.Elasticsearch) (*Client, error) {
	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: cfg.Addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
		APIKey:    cfg.APIKey,
	})
	if err != nil {
		return nil, err
	}

	return &Client{es: es, indexPrefix: cfg.IndexPrefix, batchSize: cfg.BatchSize}, nil
}

// Ping checks the index pattern the exporter writes into rather than the cluster root, which
// answers 403 for an API key holding index privileges only. A pattern matching nothing is still a
// 200, so a deployment is ready before its first scrape has created an index.
func (c *Client) Ping(ctx context.Context) error {
	res, err := c.es.Indices.Exists(
		[]string{c.indexPrefix + "-*"},
		c.es.Indices.Exists.WithContext(ctx),
		c.es.Indices.Exists.WithAllowNoIndices(true),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return responseError(res.Status(), res.Body)
	}

	return nil
}

func (c *Client) Close(context.Context) error { return nil }

func (c *Client) index(doc storage.Document) string {
	return fmt.Sprintf("%s-%s-%s", c.indexPrefix, doc.ResourceType, doc.ScrapedAt.UTC().Format(indexDateLayout))
}

func responseError(status string, body io.Reader) error {
	b, _ := io.ReadAll(io.LimitReader(body, 2048))
	if len(bytes.TrimSpace(b)) == 0 {
		return fmt.Errorf("elasticsearch: %s", status)
	}
	return fmt.Errorf("elasticsearch: %s: %s", status, b)
}
