package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/windkube/aws-metrics-exporter/internal/storage"
)

const maxReportedFailures = 5

type envelope struct {
	Timestamp time.Time       `json:"@timestamp"`
	Resource  envelopeRes     `json:"resource"`
	Cloud     envelopeCloud   `json:"cloud"`
	AWS       json.RawMessage `json:"aws"`
}

type envelopeRes struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type envelopeCloud struct {
	Provider string               `json:"provider"`
	Region   string               `json:"region"`
	Account  envelopeCloudAccount `json:"account"`
}

type envelopeCloudAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Write indexes docs synchronously and fails if any individual item was rejected, so the caller
// can retry the whole batch. Document IDs are stable, which makes that retry an overwrite.
func (c *Client) Write(ctx context.Context, docs []storage.Document) error {
	if len(docs) == 0 {
		return nil
	}

	var body bytes.Buffer
	for _, doc := range docs {
		meta, err := json.Marshal(map[string]any{
			"index": map[string]string{"_index": c.index(doc), "_id": doc.ID()},
		})
		if err != nil {
			return err
		}

		source, err := json.Marshal(envelope{
			Timestamp: doc.ScrapedAt.UTC(),
			Resource:  envelopeRes{Type: doc.ResourceType, ID: doc.ResourceID, Name: doc.ResourceName},
			Cloud: envelopeCloud{
				Provider: "aws",
				Region:   doc.Region,
				Account:  envelopeCloudAccount{ID: doc.AccountID, Name: doc.AccountName},
			},
			AWS: doc.Data,
		})
		if err != nil {
			return err
		}

		body.Write(meta)
		body.WriteByte('\n')
		body.Write(source)
		body.WriteByte('\n')
	}

	res, err := c.es.Bulk(&body, c.es.Bulk.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return responseError(res.Status(), res.Body)
	}

	var parsed struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			ID     string `json:"_id"`
			Status int    `json:"status"`
			Error  *struct {
				Type   string `json:"type"`
				Reason string `json:"reason"`
			} `json:"error"`
		} `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return fmt.Errorf("elasticsearch: decoding bulk response: %w", err)
	}

	if !parsed.Errors {
		return nil
	}

	var (
		failed  int
		reasons []string
	)
	for _, item := range parsed.Items {
		for _, result := range item {
			if result.Error == nil {
				continue
			}
			failed++
			if len(reasons) < maxReportedFailures {
				reasons = append(reasons, fmt.Sprintf("%s: %s: %s", result.ID, result.Error.Type, result.Error.Reason))
			}
		}
	}

	return fmt.Errorf("elasticsearch: %d of %d documents rejected: %s",
		failed, len(docs), strings.Join(reasons, "; "))
}
