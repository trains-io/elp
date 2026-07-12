package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// LabelDeviceName labels CANDetector objects with their parent Z21Device name.
	LabelDeviceName = "z21.trains.io/device"
)

// CANDetectorPhase reports address allocation state.
type CANDetectorPhase string

const (
	CANDetectorPhasePending   CANDetectorPhase = "Pending"
	CANDetectorPhaseAllocated CANDetectorPhase = "Allocated"
	CANDetectorPhaseApplied   CANDetectorPhase = "Applied"
	CANDetectorPhaseFailed    CANDetectorPhase = "Failed"
)

// CANDetectorSpec defines a CAN occupancy detector discovered on a Z21 command station.
type CANDetectorSpec struct {
	// DeviceRef names the Z21Device that owns this detector (same namespace).
	DeviceRef corev1.LocalObjectReference `json:"deviceRef"`

	// NetID is the permanent CAN network identifier (for example 0xDB04).
	// +kubebuilder:validation:Minimum=1
	NetID uint16 `json:"netID"`

	// DesiredAddress is the module address the allocator selected (1-255).
	// Written by the CANDetector controller; do not set manually unless adopting.
	// +optional
	DesiredAddress *uint16 `json:"desiredAddress,omitempty"`

	// AdoptObserved requests keeping the hardware module address when it already
	// lies in the configured pool and is unassigned.
	// +kubebuilder:default=true
	// +optional
	AdoptObserved *bool `json:"adoptObserved,omitempty"`
}

// CANDetectorStatus defines observed detector state.
type CANDetectorStatus struct {
	// ObservedAddress is the module address last read from the Z21 (LAN_CAN_DETECTOR).
	// +optional
	ObservedAddress *uint16 `json:"observedAddress,omitempty"`

	// AssignedAddress is the address reserved by the pool allocator for this detector.
	// +optional
	AssignedAddress *uint16 `json:"assignedAddress,omitempty"`

	// Phase is the allocation lifecycle state.
	// +optional
	Phase CANDetectorPhase `json:"phase,omitempty"`

	// Message explains Pending or Failed state.
	// +optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=candet
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="NetID",type=string,JSONPath=`.spec.netID`,priority=0
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.spec.desiredAddress`
// +kubebuilder:printcolumn:name="Observed",type=integer,JSONPath=`.status.observedAddress`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CANDetector represents one CAN occupancy detector module on a Z21 command station.
type CANDetector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CANDetectorSpec   `json:"spec,omitempty"`
	Status CANDetectorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CANDetectorList contains a list of CANDetector.
type CANDetectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CANDetector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CANDetector{}, &CANDetectorList{})
}

// AdoptObservedEnabled returns whether the detector should prefer its observed address.
func (d *CANDetector) AdoptObservedEnabled() bool {
	if d.Spec.AdoptObserved == nil {
		return true
	}
	return *d.Spec.AdoptObserved
}

// OwnerKey returns a stable pool owner identity for this detector CR.
func (d *CANDetector) OwnerKey() string {
	return d.Namespace + "/" + d.Name
}
