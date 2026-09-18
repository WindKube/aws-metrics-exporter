package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
	"github.com/windkube/aws-metrics-exporter/internal/config"
	"github.com/windkube/aws-metrics-exporter/internal/storage"
)

// ConfigFactory hands out AWS configs that have assumed the account's exporter role.
type ConfigFactory interface {
	Config(account config.Account, region string) aws.Config
}

// Scraper collects one resource type from one account and region and writes it straight to the
// store. Objects are never returned to the caller, which keeps them out of Temporal's history.
type Scraper struct {
	factory   ConfigFactory
	registry  *collector.Registry
	store     storage.Store
	batchSize int
}

func New(factory ConfigFactory, registry *collector.Registry, store storage.Store, batchSize int) *Scraper {
	return &Scraper{factory: factory, registry: registry, store: store, batchSize: batchSize}
}

type Request struct {
	Account      config.Account
	ResourceType string
	Region       string
	ScrapedAt    time.Time
}

type Result struct {
	Count   int `json:"count"`
	Batches int `json:"batches"`
}

// Scrape calls progress after every flushed batch with the number of objects written so far.
func (s *Scraper) Scrape(ctx context.Context, req Request, progress func(int)) (Result, error) {
	c, err := s.registry.Get(req.ResourceType)
	if err != nil {
		return Result{}, err
	}

	region := req.Region
	if c.Scope() == collector.ScopeGlobal {
		region = collector.GlobalRegion
	}

	var (
		result Result
		batch  = make([]storage.Document, 0, s.batchSize)
	)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := s.store.Write(ctx, batch); err != nil {
			return err
		}
		result.Batches++
		batch = batch[:0]
		if progress != nil {
			progress(result.Count)
		}
		return nil
	}

	emit := func(r collector.Resource) error {
		data, err := json.Marshal(r.Data)
		if err != nil {
			return fmt.Errorf("marshalling %s %s: %w", req.ResourceType, r.ID, err)
		}

		batch = append(batch, storage.Document{
			ResourceType: req.ResourceType,
			ResourceID:   r.ID,
			ResourceName: r.Name,
			AccountID:    req.Account.ID,
			AccountName:  req.Account.Name,
			Region:       region,
			ScrapedAt:    req.ScrapedAt,
			Data:         data,
		})
		result.Count++

		if len(batch) >= s.batchSize {
			return flush()
		}
		return nil
	}

	if err := c.Collect(ctx, s.factory.Config(req.Account, region), emit); err != nil {
		return result, err
	}

	return result, flush()
}
