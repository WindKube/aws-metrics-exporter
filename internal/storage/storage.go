package storage

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// Document is one scraped AWS object. Data holds the object's own AWS JSON representation; the
// remaining fields are the envelope the backend indexes it under.
type Document struct {
	ResourceType string
	ResourceID   string
	ResourceName string
	AccountID    string
	AccountName  string
	Region       string
	ScrapedAt    time.Time
	Data         json.RawMessage
}

// ID is stable for a given object, so re-running a scrape overwrites rather than duplicates.
func (d Document) ID() string {
	return strings.Join([]string{d.AccountID, d.Region, d.ResourceType, d.ResourceID}, ":")
}

type Store interface {
	Write(ctx context.Context, docs []Document) error
	Ping(ctx context.Context) error
	Close(ctx context.Context) error
}
