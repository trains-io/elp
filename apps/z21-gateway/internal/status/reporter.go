package status

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

// SessionUpdate is written to Z21Device status by the gateway.
type SessionUpdate struct {
	LastSeen            time.Time
	SerialNumber        string
	FirmwareVersion     string
	HardwareType        string
	XBusVersion         string
	XBusFirmwareVersion string
	LockCode            *uint8
}

// HealthUpdate is written to Z21Device status by the gateway health loop.
type HealthUpdate struct {
	DeviceReachable bool
	Degraded        bool
	Failures        int32
	LastSuccess     *time.Time
}

// Reporter patches Z21Device status from the gateway.
type Reporter struct {
	client    client.Client
	namespace string
	name      string
}

func NewReporter(namespace, name string) (*Reporter, error) {
	cfg, err := loadInClusterConfig()
	if err != nil {
		return nil, err
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(z21v1alpha1.AddToScheme(scheme))

	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	return &Reporter{
		client:    cl,
		namespace: namespace,
		name:      name,
	}, nil
}

// Client returns the underlying Kubernetes client.
func (r *Reporter) Client() client.Client {
	return r.client
}

func (r *Reporter) deviceKey() types.NamespacedName {
	return types.NamespacedName{Namespace: r.namespace, Name: r.name}
}

func (r *Reporter) UpdateSession(ctx context.Context, update SessionUpdate) error {
	device := &z21v1alpha1.Z21Device{}
	if err := r.client.Get(ctx, r.deviceKey(), device); err != nil {
		return err
	}

	patchBase := device.DeepCopy()
	ts := metav1.NewTime(update.LastSeen)
	device.Status.LastSeen = &ts
	if update.SerialNumber != "" {
		device.Status.SerialNumber = update.SerialNumber
	}
	if update.FirmwareVersion != "" {
		device.Status.FirmwareVersion = update.FirmwareVersion
	}
	if update.HardwareType != "" {
		device.Status.HardwareType = update.HardwareType
	}
	if update.XBusVersion != "" {
		device.Status.XBusVersion = update.XBusVersion
	}
	if update.XBusFirmwareVersion != "" {
		device.Status.XBusFirmwareVersion = update.XBusFirmwareVersion
	}
	if update.LockCode != nil {
		device.Status.LockCode = update.LockCode
	}
	return r.client.Status().Patch(ctx, device, client.MergeFrom(patchBase))
}

// HealthCheckSpec reads the effective health check configuration from the Z21Device spec.
func (r *Reporter) HealthCheckSpec(ctx context.Context) (z21v1alpha1.HealthCheckSpec, error) {
	device := &z21v1alpha1.Z21Device{}
	if err := r.client.Get(ctx, r.deviceKey(), device); err != nil {
		return z21v1alpha1.HealthCheckSpec{}, err
	}
	return device.HealthCheckSpecOrDefaults(), nil
}

// UpdateHealth patches reachability conditions and health probe counters.
func (r *Reporter) UpdateHealth(ctx context.Context, update HealthUpdate) error {
	device := &z21v1alpha1.Z21Device{}
	if err := r.client.Get(ctx, r.deviceKey(), device); err != nil {
		return err
	}

	patchBase := device.DeepCopy()
	device.Status.HealthCheckFailures = update.Failures
	if update.LastSuccess != nil {
		ts := metav1.NewTime(*update.LastSuccess)
		device.Status.LastSuccessfulHealthCheck = &ts
	}

	setReachabilityCondition(device, update.DeviceReachable, update.Failures)
	setDegradedCondition(device, update.Degraded)

	return r.client.Status().Patch(ctx, device, client.MergeFrom(patchBase))
}

func setReachabilityCondition(device *z21v1alpha1.Z21Device, reachable bool, failures int32) {
	status := metav1.ConditionFalse
	reason := "ProbeFailing"
	message := fmt.Sprintf("Z21 endpoint failed %d consecutive health probe(s)", failures)
	if reachable {
		status = metav1.ConditionTrue
		reason = "ProbeSucceeded"
		message = "Z21 endpoint responded to health probe"
	}
	meta.SetStatusCondition(&device.Status.Conditions, metav1.Condition{
		Type:               z21v1alpha1.ConditionDeviceReachable,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: device.Generation,
	})
}

func setDegradedCondition(device *z21v1alpha1.Z21Device, degraded bool) {
	status := metav1.ConditionFalse
	reason := "Healthy"
	message := "Z21 endpoint is reachable"
	if degraded {
		status = metav1.ConditionTrue
		reason = "DeviceUnreachable"
		message = "Z21 endpoint is unreachable"
	}
	meta.SetStatusCondition(&device.Status.Conditions, metav1.Condition{
		Type:               z21v1alpha1.ConditionDegraded,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: device.Generation,
	})
}

// BroadcastFlags reads the desired broadcast flags from the Z21Device spec.
func (r *Reporter) BroadcastFlags(ctx context.Context) (uint32, error) {
	device := &z21v1alpha1.Z21Device{}
	if err := r.client.Get(ctx, r.deviceKey(), device); err != nil {
		return 0, err
	}
	return device.BroadcastFlagsValue(), nil
}
