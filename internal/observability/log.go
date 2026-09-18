package observability

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	temporallog "go.temporal.io/sdk/log"

	"github.com/windkube/aws-metrics-exporter/internal/config"
)

func ConfigureLogging(cfg config.Log, extra ...io.Writer) error {
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		return err
	}

	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = time.RFC3339

	var out io.Writer = os.Stderr
	if cfg.Format == "console" {
		out = zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
	}

	writers := []io.Writer{out}
	for _, w := range extra {
		if w != nil {
			writers = append(writers, w)
		}
	}
	if len(writers) > 1 {
		out = zerolog.MultiLevelWriter(writers...)
	}

	log.Logger = zerolog.New(out).With().Timestamp().Logger()

	return nil
}

// TemporalLogger adapts the SDK's key-value logger onto the global zerolog logger.
func TemporalLogger() temporallog.Logger { return temporalLogger{} }

type temporalLogger struct{}

func (temporalLogger) Debug(msg string, keyvals ...any) { emit(log.Debug(), msg, keyvals) }
func (temporalLogger) Info(msg string, keyvals ...any)  { emit(log.Info(), msg, keyvals) }
func (temporalLogger) Warn(msg string, keyvals ...any)  { emit(log.Warn(), msg, keyvals) }
func (temporalLogger) Error(msg string, keyvals ...any) { emit(log.Error(), msg, keyvals) }

func emit(event *zerolog.Event, msg string, keyvals []any) {
	for i := 0; i+1 < len(keyvals); i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			continue
		}
		event = event.Interface(key, keyvals[i+1])
	}
	event.Msg(msg)
}
