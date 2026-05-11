package main

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/sourcegraph/conc/pool"
	"go.uber.org/zap"
)

type syncResult struct {
	dest string
	err  error
}

// Syncer performs rsync from source to replica directories.
type Syncer struct {
	cfg *Config
	mu  sync.Mutex // serializes doSync calls; guards startAt

	muState  sync.RWMutex
	running  bool
	startAt  time.Time
	lastAt   time.Time
	duration time.Duration
	errors   []string
}

// setErrors replaces the stored errors under muState write lock.
func (s *Syncer) setErrors(errs []string) {
	s.muState.Lock()
	s.errors = errs
	s.muState.Unlock()
}

func NewSyncer(cfg *Config) *Syncer {
	return &Syncer{cfg: cfg}
}

// SyncState is the current sync status for /status endpoint.
type SyncState struct {
	Running      bool     `json:"running"`
	LastSyncAt   string   `json:"last_sync_at,omitempty"`
	LastDuration string   `json:"last_duration,omitempty"`
	LastErrors   []string `json:"last_errors,omitempty"`
}

// State returns a snapshot of the current sync status.
func (s *Syncer) State() SyncState {
	s.muState.RLock()
	defer s.muState.RUnlock()
	st := SyncState{
		Running:    s.running,
		LastErrors: s.errors,
	}
	if !s.lastAt.IsZero() {
		st.LastSyncAt = s.lastAt.UTC().Format(time.RFC3339)
	}
	if s.duration > 0 {
		st.LastDuration = s.duration.Round(time.Millisecond).String()
	}
	if st.LastErrors == nil {
		st.LastErrors = []string{}
	}
	return st
}

// TryStart attempts to start a sync in the background.
// Returns false if a sync is already running.
// The sync goroutine uses context.WithoutCancel to survive caller cancellation
// (e.g. client disconnect or ticker context expiry) while preserving context values.
func (s *Syncer) TryStart(ctx context.Context) bool {
	if !s.mu.TryLock() {
		return false
	}
	go func() {
		defer s.mu.Unlock()
		s.doSync(context.WithoutCancel(ctx))
	}()
	return true
}

// doSync performs rsync to all replica directories concurrently.
func (s *Syncer) doSync(ctx context.Context) {
	s.muState.Lock()
	s.running = true
	s.startAt = time.Now()
	s.muState.Unlock()

	defer func() {
		s.muState.Lock()
		s.running = false
		s.lastAt = time.Now()
		s.duration = time.Since(s.startAt)
		s.muState.Unlock()
	}()

	// Validate source directory once before syncing to all destinations.
	srcInfo, err := os.Stat(s.cfg.SyncSource)
	if err != nil {
		zap.L().Error("sync aborted: source dir stat failed", zap.Error(err))
		s.setErrors([]string{fmt.Sprintf("source stat failed: %v", err)})
		return
	}
	if !srcInfo.IsDir() {
		msg := fmt.Sprintf("source %s is not a directory", s.cfg.SyncSource)
		zap.L().Error("sync aborted", zap.String("reason", msg))
		s.setErrors([]string{msg})
		return
	}

	p := pool.NewWithResults[syncResult]().WithContext(ctx)
	for _, dest := range s.cfg.SyncDests {
		dest := dest
		p.Go(func(poolCtx context.Context) (syncResult, error) {
			return s.syncToDest(poolCtx, dest), nil
		})
	}
	results, _ := p.Wait()

	var errs []string
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", r.dest, r.err))
		}
	}
	s.setErrors(errs)

	if len(errs) > 0 {
		zap.L().Warn("sync completed with errors", zap.Strings("errors", errs))
	} else {
		zap.L().Info("sync completed successfully", zap.Int("dests", len(results)))
	}
}

func (s *Syncer) syncToDest(ctx context.Context, dest string) syncResult {
	if err := validateDestURL(dest); err != nil {
		return syncResult{dest: dest, err: err}
	}

	start := time.Now()

	args := []string{"-avz", "--delete", "--times"}
	for _, excl := range s.cfg.SyncExcludes {
		args = append(args, "--exclude="+excl)
	}
	args = append(args, s.cfg.SyncSource, dest)

	zap.L().Info("rsync starting",
		zap.String("source", s.cfg.SyncSource),
		zap.String("dest", dest),
	)

	cmd := exec.CommandContext(ctx, "rsync", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	if err != nil {
		return syncResult{
			dest: dest,
			err:  fmt.Errorf("rsync failed in %v (stderr: %s)", duration, stderr.String()),
		}
	}

	zap.L().Info("rsync completed",
		zap.String("dest", dest),
		zap.Duration("duration", duration),
	)

	return syncResult{dest: dest}
}

func validateDestURL(dest string) error {
	u, err := url.Parse(dest)
	if err != nil {
		return fmt.Errorf("invalid destination URL: %w", err)
	}
	if u.Scheme != "rsync" {
		return fmt.Errorf("destination must be rsync:// URL, got %s://", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("rsync URL missing host: %s", dest)
	}
	if u.Path == "" {
		return fmt.Errorf("rsync URL missing module path: %s", dest)
	}
	return nil
}
