package status

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

func TestHealthTrackerSuccessThreshold(t *testing.T) {
	t.Parallel()

	tracker := newHealthTracker()
	spec := z21v1alpha1.HealthCheckSpec{
		FailureThreshold: 3,
		SuccessThreshold: 1,
	}
	now := time.Now()

	update, changed := tracker.record(true, now, spec)
	if !changed || !update.DeviceReachable || update.Degraded || update.Failures != 0 {
		t.Fatalf("first success = %#v changed=%v", update, changed)
	}
	if update.LastSuccess == nil {
		t.Fatal("expected last success timestamp")
	}

	update, changed = tracker.record(true, now.Add(time.Second), spec)
	if !changed || !update.DeviceReachable || update.Degraded {
		t.Fatalf("second success = %#v changed=%v", update, changed)
	}
}

func TestHealthTrackerFailureThreshold(t *testing.T) {
	t.Parallel()

	tracker := newHealthTracker()
	spec := z21v1alpha1.HealthCheckSpec{
		FailureThreshold: 3,
		SuccessThreshold: 1,
	}
	now := time.Now()

	tracker.record(true, now, spec)
	tracker.record(true, now.Add(time.Second), spec)

	for i := int32(1); i < 3; i++ {
		update, changed := tracker.record(false, now.Add(time.Duration(i)*time.Second), spec)
		if !changed || !update.DeviceReachable || update.Degraded || update.Failures != i {
			t.Fatalf("failure %d = %#v changed=%v", i, update, changed)
		}
	}

	update, changed := tracker.record(false, now.Add(4*time.Second), spec)
	if !changed || update.DeviceReachable || !update.Degraded || update.Failures != 3 {
		t.Fatalf("third failure = %#v changed=%v", update, changed)
	}
}

func TestHealthTrackerRecoversAfterFailures(t *testing.T) {
	t.Parallel()

	tracker := newHealthTracker()
	spec := z21v1alpha1.HealthCheckSpec{
		FailureThreshold: 3,
		SuccessThreshold: 1,
	}
	now := time.Now()

	for range 3 {
		tracker.record(false, now, spec)
	}

	update, changed := tracker.record(true, now.Add(time.Second), spec)
	if !changed || update.Failures != 0 || !update.DeviceReachable || update.Degraded {
		t.Fatalf("recovery = %#v changed=%v", update, changed)
	}
}

func TestHealthTrackerUsesDefaults(t *testing.T) {
	t.Parallel()

	tracker := newHealthTracker()
	spec := z21v1alpha1.HealthCheckSpec{}
	now := time.Now()

	_, _ = tracker.record(true, now, spec)
	update, changed := tracker.record(true, now.Add(time.Second), spec)
	if !changed || !update.DeviceReachable {
		t.Fatalf("default success threshold = %#v changed=%v", update, changed)
	}
}

func TestHealthTrackerTransientFailureWhileReachable(t *testing.T) {
	t.Parallel()

	tracker := newHealthTracker()
	spec := z21v1alpha1.HealthCheckSpec{
		FailureThreshold: 3,
		SuccessThreshold: 1,
	}
	now := time.Now()

	tracker.record(true, now, spec)
	tracker.record(true, now.Add(time.Second), spec)

	update, changed := tracker.record(false, now.Add(2*time.Second), spec)
	if !changed || !update.DeviceReachable || update.Degraded || update.Failures != 1 {
		t.Fatalf("transient failure = %#v changed=%v", update, changed)
	}
}

func TestHealthTrackerDefaultIntervalFallback(t *testing.T) {
	t.Parallel()

	if z21v1alpha1.DefaultHealthCheckInterval.Duration != 2*time.Second {
		t.Fatalf("interval = %v", z21v1alpha1.DefaultHealthCheckInterval.Duration)
	}
	if z21v1alpha1.DefaultHealthCheckTimeout.Duration != 500*time.Millisecond {
		t.Fatalf("timeout = %v", z21v1alpha1.DefaultHealthCheckTimeout.Duration)
	}
}

func TestSetReachabilityCondition(t *testing.T) {
	t.Parallel()

	device := &z21v1alpha1.Z21Device{
		ObjectMeta: metav1.ObjectMeta{Generation: 2},
	}
	setReachabilityCondition(device, true, 0)
	cond := device.Status.Conditions[0]
	if cond.Type != z21v1alpha1.ConditionDeviceReachable || cond.Status != metav1.ConditionTrue {
		t.Fatalf("condition = %#v", cond)
	}
}
