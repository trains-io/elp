package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CANAddressPoolSpec defines an allocatable module address range.
type CANAddressPoolSpec struct {
	// Start is the first auto-allocatable module address (inclusive).
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=255
	Start uint16 `json:"start"`

	// End is the last auto-allocatable module address (inclusive).
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=255
	End uint16 `json:"end"`

	// Reserved addresses are never auto-allocated; they may still be claimed explicitly.
	// +optional
	Reserved []uint16 `json:"reserved,omitempty"`
}

// CANAddressPoolStatus summarizes pool usage (aggregated by the CANDetector controller).
type CANAddressPoolStatus struct {
	// RangeSize is the number of addresses between start and end inclusive.
	// +optional
	RangeSize int `json:"rangeSize,omitempty"`

	// Reserved is the number of reserved addresses in spec.
	// +optional
	Reserved int `json:"reserved,omitempty"`

	// Allocated is the number of addresses currently assigned to detectors.
	// +optional
	Allocated int `json:"allocated,omitempty"`

	// AutoAvailable is the number of addresses Allocate may still choose.
	// +optional
	AutoAvailable int `json:"autoAvailable,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=canpool
// +kubebuilder:printcolumn:name="Range",type=string,JSONPath=`.spec.start`,priority=0
// +kubebuilder:printcolumn:name="End",type=string,JSONPath=`.spec.end`
// +kubebuilder:printcolumn:name="Allocated",type=integer,JSONPath=`.status.allocated`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CANAddressPool is a module address range shared by detectors on one layout.
type CANAddressPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CANAddressPoolSpec   `json:"spec,omitempty"`
	Status CANAddressPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CANAddressPoolList contains a list of CANAddressPool.
type CANAddressPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CANAddressPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CANAddressPool{}, &CANAddressPoolList{})
}

// PoolConfig returns the canpool library configuration for this CR.
func (p *CANAddressPool) PoolConfig() (start, end uint16, reserved []uint16) {
	return p.Spec.Start, p.Spec.End, append([]uint16(nil), p.Spec.Reserved...)
}
