package scraper_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
	"github.com/windkube/aws-metrics-exporter/internal/config"
	"github.com/windkube/aws-metrics-exporter/internal/scraper"
	"github.com/windkube/aws-metrics-exporter/internal/storage"
)

type stubFactory struct{ regions []string }

func (f *stubFactory) Config(_ config.Account, region string) aws.Config {
	f.regions = append(f.regions, region)
	return aws.Config{}
}

type fakeCollector struct {
	resourceType string
	scope        collector.Scope
	count        int
	err          error
}

func (c fakeCollector) Type() string           { return c.resourceType }
func (c fakeCollector) Scope() collector.Scope { return c.scope }

func (c fakeCollector) Collect(_ context.Context, _ aws.Config, emit collector.EmitFunc) error {
	for i := range c.count {
		if err := emit(collector.Resource{
			ID:   fmt.Sprintf("arn:%d", i),
			Name: fmt.Sprintf("object-%d", i),
			Data: map[string]string{"name": fmt.Sprintf("object-%d", i)},
		}); err != nil {
			return err
		}
	}
	return c.err
}

type memoryStore struct {
	batches [][]storage.Document
	err     error
}

func (s *memoryStore) Write(_ context.Context, docs []storage.Document) error {
	if s.err != nil {
		return s.err
	}
	s.batches = append(s.batches, append([]storage.Document(nil), docs...))
	return nil
}

func (s *memoryStore) Ping(context.Context) error  { return nil }
func (s *memoryStore) Close(context.Context) error { return nil }

func request() scraper.Request {
	return scraper.Request{
		Account:      config.Account{Name: "prod", ID: "111111111111"},
		ResourceType: "fake",
		Region:       "eu-west-1",
		ScrapedAt:    time.Date(2026, 9, 18, 1, 0, 0, 0, time.UTC),
	}
}

func TestScrapeFlushesFullBatchesAndReportsProgress(t *testing.T) {
	store := &memoryStore{}
	s := scraper.New(
		&stubFactory{},
		collector.NewRegistry(fakeCollector{resourceType: "fake", scope: collector.ScopeRegional, count: 5}),
		store,
		2,
	)

	var progress []int
	result, err := s.Scrape(t.Context(), request(), func(count int) { progress = append(progress, count) })
	require.NoError(t, err)

	assert.Equal(t, 5, result.Count)
	assert.Equal(t, 3, result.Batches)
	assert.Equal(t, []int{2, 4, 5}, progress)

	require.Len(t, store.batches, 3)
	assert.Len(t, store.batches[0], 2)
	assert.Len(t, store.batches[2], 1)

	doc := store.batches[0][0]
	assert.Equal(t, "111111111111:eu-west-1:fake:arn:0", doc.ID())
	assert.Equal(t, "prod", doc.AccountName)
	assert.JSONEq(t, `{"name":"object-0"}`, string(doc.Data))
}

func TestScrapeForcesTheGlobalRegionOnGlobalCollectors(t *testing.T) {
	factory := &stubFactory{}
	store := &memoryStore{}
	s := scraper.New(
		factory,
		collector.NewRegistry(fakeCollector{resourceType: "fake", scope: collector.ScopeGlobal, count: 1}),
		store,
		10,
	)

	_, err := s.Scrape(t.Context(), request(), nil)
	require.NoError(t, err)

	assert.Equal(t, []string{collector.GlobalRegion}, factory.regions)
	assert.Equal(t, collector.GlobalRegion, store.batches[0][0].Region)
}

func TestScrapeFailsOnUnknownResourceType(t *testing.T) {
	s := scraper.New(&stubFactory{}, collector.NewRegistry(), &memoryStore{}, 10)

	_, err := s.Scrape(t.Context(), request(), nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown resource type "fake"`)
}

func TestScrapeReturnsTheStoreFailure(t *testing.T) {
	sentinel := errors.New("elasticsearch unavailable")
	s := scraper.New(
		&stubFactory{},
		collector.NewRegistry(fakeCollector{resourceType: "fake", scope: collector.ScopeRegional, count: 3}),
		&memoryStore{err: sentinel},
		2,
	)

	_, err := s.Scrape(t.Context(), request(), nil)

	require.ErrorIs(t, err, sentinel)
}
