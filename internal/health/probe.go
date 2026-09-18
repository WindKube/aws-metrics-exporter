package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// failureThreshold tolerates a single blip so a restarting Elasticsearch node does not immediately
// pull every pod out of rotation.
const failureThreshold = 2

type Check struct {
	Name string
	Func func(context.Context) error
}

type Prober struct {
	checks   []Check
	interval time.Duration

	mu       sync.RWMutex
	ready    bool
	failures int
	lastErr  error
}

func NewProber(interval time.Duration, checks ...Check) *Prober {
	return &Prober{checks: checks, interval: interval}
}

func (p *Prober) Ready() (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ready, p.lastErr
}

// Run probes until ctx is cancelled, starting with an immediate probe. A prober reports not ready
// until that first probe succeeds.
func (p *Prober) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		p.Probe(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *Prober) Probe(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, p.interval)
	defer cancel()

	var err error
	for _, check := range p.checks {
		if failure := check.Func(ctx); failure != nil {
			err = fmt.Errorf("%s: %w", check.Name, failure)
			break
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if err == nil {
		p.failures = 0
		p.ready = true
		p.lastErr = nil
		return
	}

	p.failures++
	p.lastErr = err
	if p.failures >= failureThreshold {
		p.ready = false
	}
}
