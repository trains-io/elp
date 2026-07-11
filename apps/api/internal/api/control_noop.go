package api

import (
	"context"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

type noopControl struct{}

// NoopControl returns a control publisher that does nothing (Commit 3).
// Replace with the real NATS publisher in Commit 5.
func NoopControl() DeviceControlPublisher {
	return noopControl{}
}

func (noopControl) PublishSetBroadcastFlags(_ context.Context, _ *z21v1alpha1.Z21Device) error {
	return nil
}
