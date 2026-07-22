package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SimulationPhase reports simulator apply state.
type SimulationPhase string

const (
	SimulationPhasePending SimulationPhase = "Pending"
	SimulationPhaseApplied SimulationPhase = "Applied"
	SimulationPhaseFailed  SimulationPhase = "Failed"
)

// SimulationCANDeviceKind selects a simulated CAN module type.
type SimulationCANDeviceKind string

const (
	SimulationCANDeviceDetector SimulationCANDeviceKind = "detector"
	SimulationCANDeviceBooster  SimulationCANDeviceKind = "booster"
)

// SimulationCANDetector declares a CAN occupancy detector module on the simulator.
type SimulationCANDetector struct {
	// NetID is the permanent CAN network identifier (for example 0xDB04).
	// +kubebuilder:validation:Minimum=1
	NetID uint16 `json:"netID"`

	// Name is an optional module label reported by the simulator.
	// +optional
	Name string `json:"name,omitempty"`

	// ModuleAddress is the 1-based module address (default 1).
	// +optional
	ModuleAddress uint16 `json:"moduleAddress,omitempty"`

	// PortCount is the number of feedback ports (default 8).
	// +optional
	PortCount uint16 `json:"portCount,omitempty"`
}

// SimulationCANBooster declares a CAN booster module on the simulator.
type SimulationCANBooster struct {
	// NetID is the permanent CAN network identifier.
	// +kubebuilder:validation:Minimum=1
	NetID uint16 `json:"netID"`

	// Name is an optional module label reported by the simulator.
	// +optional
	Name string `json:"name,omitempty"`

	// OutputPort is the booster output index (default 1).
	// +optional
	OutputPort uint16 `json:"outputPort,omitempty"`
}

// SimulationSpec defines desired simulator state for one Z21Device backend.
type SimulationSpec struct {
	// DeviceRef names the Z21Device with backend.type simulator (same namespace).
	DeviceRef corev1.LocalObjectReference `json:"deviceRef"`

	// CANDetectors are occupancy detector modules to register on the simulator.
	// +optional
	CANDetectors []SimulationCANDetector `json:"canDetectors,omitempty"`

	// CANBoosters are CAN booster modules to register on the simulator.
	// +optional
	CANBoosters []SimulationCANBooster `json:"canBoosters,omitempty"`
}

// SimulationStatus reports what the simulator controller last applied.
type SimulationStatus struct {
	// Phase is Pending, Applied, or Failed.
	Phase SimulationPhase `json:"phase,omitempty"`

	// Message explains a Failed phase or the last apply error.
	Message string `json:"message,omitempty"`

	// ObservedGeneration is the last reconciled metadata.generation.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// AppliedCANDevices counts detectors and boosters successfully registered.
	AppliedCANDevices int32 `json:"appliedCANDevices,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=sim
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="CAN",type=integer,JSONPath=`.status.appliedCANDevices`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Simulation drives z21-sim through the in-pod gRPC control plane.
type Simulation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SimulationSpec   `json:"spec,omitempty"`
	Status SimulationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SimulationList contains a list of Simulation.
type SimulationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Simulation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Simulation{}, &SimulationList{})
}
