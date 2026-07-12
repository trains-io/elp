package status

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

func TestUpdateHealthSetsConditions(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := z21v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	device := &z21v1alpha1.Z21Device{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "main",
			Namespace:  "default",
			Generation: 4,
		},
		Spec: z21v1alpha1.Z21DeviceSpec{
			Backend: z21v1alpha1.BackendSpec{
				Type:     z21v1alpha1.BackendHardware,
				Hardware: &z21v1alpha1.HardwareBackend{Host: "192.168.0.42"},
			},
			NATS: z21v1alpha1.NatsSpec{URL: "nats://nats:4222"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(device).
		WithStatusSubresource(&z21v1alpha1.Z21Device{}).
		Build()

	reporter := &Reporter{client: cl, namespace: "default", name: "main"}
	lastSuccess := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	if err := reporter.UpdateHealth(context.Background(), HealthUpdate{
		DeviceReachable: true,
		Degraded:        false,
		Failures:        0,
		LastSuccess:     &lastSuccess,
	}); err != nil {
		t.Fatal(err)
	}

	updated := &z21v1alpha1.Z21Device{}
	if err := cl.Get(context.Background(), reporter.deviceKey(), updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status.HealthCheckFailures != 0 {
		t.Fatalf("failures = %d", updated.Status.HealthCheckFailures)
	}
	if updated.Status.LastSuccessfulHealthCheck == nil {
		t.Fatal("expected lastSuccessfulHealthCheck")
	}
	if len(updated.Status.Conditions) != 2 {
		t.Fatalf("conditions = %#v", updated.Status.Conditions)
	}

	reachable := findCondition(updated.Status.Conditions, z21v1alpha1.ConditionDeviceReachable)
	if reachable == nil || reachable.Status != metav1.ConditionTrue {
		t.Fatalf("DeviceReachable = %#v", reachable)
	}
	degraded := findCondition(updated.Status.Conditions, z21v1alpha1.ConditionDegraded)
	if degraded == nil || degraded.Status != metav1.ConditionFalse {
		t.Fatalf("Degraded = %#v", degraded)
	}
}

func TestHealthCheckSpecOrDefaultsFromCR(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := z21v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	device := &z21v1alpha1.Z21Device{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
		Spec: z21v1alpha1.Z21DeviceSpec{
			Backend: z21v1alpha1.BackendSpec{
				Type:     z21v1alpha1.BackendHardware,
				Hardware: &z21v1alpha1.HardwareBackend{Host: "192.168.0.42"},
			},
			NATS: z21v1alpha1.NatsSpec{URL: "nats://nats:4222"},
			HealthCheck: z21v1alpha1.HealthCheckSpec{
				Interval:         metav1.Duration{Duration: 5 * time.Second},
				FailureThreshold: 5,
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(device).Build()
	reporter := &Reporter{client: cl, namespace: "default", name: "main"}

	spec, err := reporter.HealthCheckSpec(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if spec.Interval.Duration != 5*time.Second {
		t.Fatalf("interval = %v", spec.Interval.Duration)
	}
	if spec.Timeout.Duration != z21v1alpha1.DefaultHealthCheckTimeout.Duration {
		t.Fatalf("timeout = %v", spec.Timeout.Duration)
	}
	if spec.FailureThreshold != 5 {
		t.Fatalf("failureThreshold = %d", spec.FailureThreshold)
	}
}

func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
