package observability

import (
	"io"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	sentryzerolog "github.com/getsentry/sentry-go/zerolog"
	"github.com/rs/zerolog"
)

const sentryFlushTimeout = 2 * time.Second

// InitSentry returns a zerolog writer that ships error-level entries to Sentry, or nil when
// SENTRY_DSN is unset.
func InitSentry(version string) (io.Writer, func(), error) {
	noop := func() {}

	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		return nil, noop, nil
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:         dsn,
		Release:     ServiceName + "@" + version,
		Environment: os.Getenv("SENTRY_ENVIRONMENT"),
	})
	if err != nil {
		return nil, noop, err
	}

	writer, err := sentryzerolog.NewWithHub(sentry.CurrentHub(), sentryzerolog.Options{
		Levels:       []zerolog.Level{zerolog.ErrorLevel, zerolog.FatalLevel, zerolog.PanicLevel},
		FlushTimeout: sentryFlushTimeout,
	})
	if err != nil {
		return nil, noop, err
	}

	return writer, func() {
		_ = writer.Close()
		sentry.Flush(sentryFlushTimeout)
	}, nil
}
