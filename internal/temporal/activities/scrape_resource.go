package activities

import (
	"context"
	"errors"
	"time"

	"github.com/aws/smithy-go"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"github.com/windkube/aws-metrics-exporter/internal/config"
	"github.com/windkube/aws-metrics-exporter/internal/scraper"
)

const ScrapeResourceName = "aws.ScrapeResource"

const (
	// ErrTypePermission marks failures a retry cannot fix, so Temporal stops immediately instead
	// of hammering an account whose role is missing a permission.
	ErrTypePermission = "AwsPermissionDenied"
	ErrTypeAWS        = "AwsError"
)

type ScrapeInput struct {
	Account      config.Account `json:"account"`
	ResourceType string         `json:"resource_type"`
	Region       string         `json:"region"`
	ScrapedAt    time.Time      `json:"scraped_at"`
}

type ScrapeOutput struct {
	Count   int `json:"count"`
	Batches int `json:"batches"`
}

type Activities struct {
	scraper *scraper.Scraper
}

func New(s *scraper.Scraper) *Activities {
	return &Activities{scraper: s}
}

func (a *Activities) ScrapeResource(ctx context.Context, in ScrapeInput) (ScrapeOutput, error) {
	activity.GetLogger(ctx).Info("scraping",
		"account", in.Account.Name, "resource_type", in.ResourceType, "region", in.Region)

	result, err := a.scraper.Scrape(ctx, scraper.Request{
		Account:      in.Account,
		ResourceType: in.ResourceType,
		Region:       in.Region,
		ScrapedAt:    in.ScrapedAt,
	}, func(count int) {
		activity.RecordHeartbeat(ctx, count)
	})
	if err != nil {
		return ScrapeOutput{}, classify(err)
	}

	return ScrapeOutput{Count: result.Count, Batches: result.Batches}, nil
}

// classify flattens the error into a single application failure. The AWS SDK nests its wrapped
// errors nine deep and Temporal serialises every level, which pushes the failure past the payload
// size limit and makes it unreadable in the UI.
func classify(err error) error {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "AccessDenied", "AccessDeniedException", "UnauthorizedOperation", "InvalidClientTokenId":
			return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypePermission, nil)
		}
	}

	return temporal.NewApplicationError(err.Error(), ErrTypeAWS)
}
