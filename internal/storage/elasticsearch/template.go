package elasticsearch

import (
	"context"
	"encoding/json"
	"strings"
)

// EnsureIndexTemplate maps the envelope explicitly and the raw AWS object as a single flattened
// field. Without it the first scrape of an IAM user with inline policies dynamically maps every
// policy statement key and hits the 1000 field limit.
func (c *Client) EnsureIndexTemplate(ctx context.Context) error {
	template := map[string]any{
		"index_patterns": []string{c.indexPrefix + "-*"},
		"priority":       100,
		"template": map[string]any{
			"mappings": map[string]any{
				"properties": map[string]any{
					"@timestamp": map[string]any{"type": "date"},
					"resource": map[string]any{"properties": map[string]any{
						"type": map[string]any{"type": "keyword"},
						"id":   map[string]any{"type": "keyword"},
						"name": map[string]any{"type": "keyword"},
					}},
					"cloud": map[string]any{"properties": map[string]any{
						"provider": map[string]any{"type": "keyword"},
						"region":   map[string]any{"type": "keyword"},
						"account": map[string]any{"properties": map[string]any{
							"id":   map[string]any{"type": "keyword"},
							"name": map[string]any{"type": "keyword"},
						}},
					}},
					"aws": map[string]any{"type": "flattened"},
				},
			},
		},
	}

	body, err := json.Marshal(template)
	if err != nil {
		return err
	}

	res, err := c.es.Indices.PutIndexTemplate(
		c.indexPrefix,
		strings.NewReader(string(body)),
		c.es.Indices.PutIndexTemplate.WithContext(ctx),
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
