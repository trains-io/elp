package controller

import (
	"fmt"

	"github.com/trains-io/elp/packages/canpool"
	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

// allocationResult is the outcome of pool reconciliation for one detector.
type allocationResult struct {
	DesiredAddress  uint16
	AssignedAddress uint16
	Phase           z21v1alpha1.CANDetectorPhase
	Message         string
	Stats           canpool.Stats
}

func allocateDetectorAddress(
	cfg canpool.Config,
	detectors []z21v1alpha1.CANDetector,
	current z21v1alpha1.CANDetector,
) (allocationResult, error) {
	pool, err := canpool.New(cfg)
	if err != nil {
		return allocationResult{
			Phase:   z21v1alpha1.CANDetectorPhaseFailed,
			Message: err.Error(),
		}, nil
	}

	assignments := assignmentsFromDetectors(detectors)
	if err := pool.Rebuild(assignments); err != nil {
		return allocationResult{
			Phase:   z21v1alpha1.CANDetectorPhaseFailed,
			Message: err.Error(),
		}, nil
	}

	owner := current.OwnerKey()
	if addr, ok := pool.AddressOf(owner); ok {
		return successResult(pool, addr), nil
	}

	if current.AdoptObservedEnabled() && current.Status.ObservedAddress != nil {
		observed := *current.Status.ObservedAddress
		if pool.IsAvailable(observed) {
			if err := pool.Claim(owner, observed); err != nil {
				return allocationResult{
					Phase:   z21v1alpha1.CANDetectorPhaseFailed,
					Message: err.Error(),
				}, nil
			}
			return successResult(pool, observed), nil
		}
	}

	addr, err := pool.Allocate(owner)
	if err != nil {
		return allocationResult{
			Phase:   z21v1alpha1.CANDetectorPhaseFailed,
			Message: err.Error(),
		}, nil
	}
	return successResult(pool, addr), nil
}

func successResult(pool *canpool.Pool, addr uint16) allocationResult {
	return allocationResult{
		DesiredAddress:  addr,
		AssignedAddress: addr,
		Phase:           z21v1alpha1.CANDetectorPhaseAllocated,
		Stats:           pool.Stats(),
	}
}

func assignmentsFromDetectors(detectors []z21v1alpha1.CANDetector) []canpool.Assignment {
	out := make([]canpool.Assignment, 0, len(detectors))
	for _, det := range detectors {
		addr, ok := assignedAddress(det)
		if !ok {
			continue
		}
		out = append(out, canpool.Assignment{
			Owner:   det.OwnerKey(),
			Address: addr,
			NetID:   det.Spec.NetID,
		})
	}
	return out
}

func assignedAddress(det z21v1alpha1.CANDetector) (uint16, bool) {
	if det.Status.AssignedAddress != nil {
		return *det.Status.AssignedAddress, true
	}
	if det.Spec.DesiredAddress != nil {
		return *det.Spec.DesiredAddress, true
	}
	return 0, false
}

func poolConfigFromCR(pool *z21v1alpha1.CANAddressPool) canpool.Config {
	start, end, reserved := pool.PoolConfig()
	return canpool.Config{
		Start:    start,
		End:      end,
		Reserved: reserved,
	}
}

func poolStatusFromStats(stats canpool.Stats) z21v1alpha1.CANAddressPoolStatus {
	return z21v1alpha1.CANAddressPoolStatus{
		RangeSize:     stats.RangeSize,
		Reserved:      stats.Reserved,
		Allocated:     stats.Allocated,
		AutoAvailable: stats.AutoAvailable,
	}
}

func ensureDeviceLabel(det *z21v1alpha1.CANDetector) bool {
	if det.Labels == nil {
		det.Labels = map[string]string{}
	}
	want := det.Spec.DeviceRef.Name
	if det.Labels[z21v1alpha1.LabelDeviceName] == want {
		return false
	}
	det.Labels[z21v1alpha1.LabelDeviceName] = want
	return true
}

func uint16Ptr(v uint16) *uint16 {
	return &v
}

func formatNetID(netID uint16) string {
	return fmt.Sprintf("0x%04X", netID)
}
