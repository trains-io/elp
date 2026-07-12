package status

import (
	"context"
	"log/slog"
	"time"

	"github.com/trains-io/z21.go/client"
	"github.com/trains-io/z21.go/protocol"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

// ProbeFunc checks Z21 reachability with a short-lived UDP session.
type ProbeFunc func(ctx context.Context, address string) error

// DefaultProbe dials the Z21 endpoint and requests hardware info.
func DefaultProbe(ctx context.Context, address string) error {
	z21, err := client.Dial(address)
	if err != nil {
		return err
	}
	defer z21.Close()

	_, err = z21.Call(ctx, protocol.GetHWInfo())
	return err
}

// HealthLoop periodically probes the Z21 endpoint and patches reachability status.
type HealthLoop struct {
	reporter *Reporter
	address  string
	probe    ProbeFunc
}

func NewHealthLoop(reporter *Reporter, address string) *HealthLoop {
	return &HealthLoop{
		reporter: reporter,
		address:  address,
		probe:    DefaultProbe,
	}
}

// Run probes until ctx is cancelled.
func (h *HealthLoop) Run(ctx context.Context) {
	state := newHealthTracker()

	for {
		spec, err := h.reporter.HealthCheckSpec(ctx)
		if err != nil {
			slog.Warn("read health check spec failed", "error", err)
			if !wait(ctx, time.Second) {
				return
			}
			continue
		}

		interval := spec.Interval.Duration
		if interval <= 0 {
			interval = z21v1alpha1.DefaultHealthCheckInterval.Duration
		}

		if !h.tick(ctx, spec, state) {
			return
		}
		if !wait(ctx, interval) {
			return
		}
	}
}

func (h *HealthLoop) tick(ctx context.Context, spec z21v1alpha1.HealthCheckSpec, state *healthTracker) bool {
	timeout := spec.Timeout.Duration
	if timeout <= 0 {
		timeout = z21v1alpha1.DefaultHealthCheckTimeout.Duration
	}

	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	err := h.probe(probeCtx, h.address)
	cancel()

	now := time.Now()
	update, changed := state.record(err == nil, now, spec)
	if !changed {
		return ctx.Err() == nil
	}

	patchCtx, patchCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer patchCancel()
	if patchErr := h.reporter.UpdateHealth(patchCtx, update); patchErr != nil {
		slog.Warn("failed to patch device health status", "error", patchErr)
	}
	return ctx.Err() == nil
}

func wait(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

type healthTracker struct {
	consecutiveFailures  int32
	consecutiveSuccesses int32
	reachable            bool
	degraded             bool
}

func newHealthTracker() *healthTracker {
	return &healthTracker{}
}

func (t *healthTracker) record(success bool, at time.Time, spec z21v1alpha1.HealthCheckSpec) (HealthUpdate, bool) {
	failureThreshold := spec.FailureThreshold
	if failureThreshold <= 0 {
		failureThreshold = z21v1alpha1.DefaultHealthCheckFailureThreshold
	}
	successThreshold := spec.SuccessThreshold
	if successThreshold <= 0 {
		successThreshold = z21v1alpha1.DefaultHealthCheckSuccessThreshold
	}

	prev := HealthUpdate{
		DeviceReachable: t.reachable,
		Degraded:        t.degraded,
		Failures:        t.consecutiveFailures,
	}

	if success {
		t.consecutiveFailures = 0
		t.consecutiveSuccesses++
		prev.LastSuccess = &at
		if !t.reachable && t.consecutiveSuccesses >= successThreshold {
			t.reachable = true
			t.degraded = false
		}
	} else {
		t.consecutiveSuccesses = 0
		t.consecutiveFailures++
		if t.consecutiveFailures >= failureThreshold {
			if t.reachable {
				t.reachable = false
			}
			t.degraded = true
		}
	}

	next := HealthUpdate{
		DeviceReachable: t.reachable,
		Degraded:        t.degraded,
		Failures:        t.consecutiveFailures,
	}
	if success {
		next.LastSuccess = &at
	}

	changed := next.DeviceReachable != prev.DeviceReachable ||
		next.Degraded != prev.Degraded ||
		next.Failures != prev.Failures ||
		(success && next.LastSuccess != nil)

	return next, changed
}
