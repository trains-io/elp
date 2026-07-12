package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/trains-io/elp/packages/canpool"
	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

func TestAllocateDetectorAddressFirstFit(t *testing.T) {
	cfg := canpool.Config{Start: 10, End: 20}
	detectors := []z21v1alpha1.CANDetector{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "trains", Name: "detector-a"},
			Spec:       z21v1alpha1.CANDetectorSpec{NetID: 0xDB04, DesiredAddress: uint16Ptr(10)},
			Status:     z21v1alpha1.CANDetectorStatus{AssignedAddress: uint16Ptr(10)},
		},
	}
	current := z21v1alpha1.CANDetector{
		ObjectMeta: metav1.ObjectMeta{Namespace: "trains", Name: "detector-b"},
		Spec:       z21v1alpha1.CANDetectorSpec{NetID: 0xDB05},
	}

	result, err := allocateDetectorAddress(cfg, detectors, current)
	if err != nil {
		t.Fatal(err)
	}
	if result.Phase != z21v1alpha1.CANDetectorPhaseAllocated || result.DesiredAddress != 11 {
		t.Fatalf("result = %+v, want address 11", result)
	}
}

func TestAllocateDetectorAddressAdoptObserved(t *testing.T) {
	cfg := canpool.Config{Start: 10, End: 100}
	observed := uint16(60)
	current := z21v1alpha1.CANDetector{
		ObjectMeta: metav1.ObjectMeta{Namespace: "trains", Name: "detector-a"},
		Spec:       z21v1alpha1.CANDetectorSpec{NetID: 0xDB04, AdoptObserved: boolPtr(true)},
		Status:     z21v1alpha1.CANDetectorStatus{ObservedAddress: &observed},
	}

	result, err := allocateDetectorAddress(cfg, nil, current)
	if err != nil {
		t.Fatal(err)
	}
	if result.DesiredAddress != 60 {
		t.Fatalf("result = %+v, want adopted address 60", result)
	}
}

func TestAllocateDetectorAddressIdempotent(t *testing.T) {
	cfg := canpool.Config{Start: 10, End: 20}
	current := z21v1alpha1.CANDetector{
		ObjectMeta: metav1.ObjectMeta{Namespace: "trains", Name: "detector-a"},
		Spec:       z21v1alpha1.CANDetectorSpec{NetID: 0xDB04, DesiredAddress: uint16Ptr(12)},
		Status:     z21v1alpha1.CANDetectorStatus{AssignedAddress: uint16Ptr(12)},
	}

	result, err := allocateDetectorAddress(cfg, []z21v1alpha1.CANDetector{current}, current)
	if err != nil {
		t.Fatal(err)
	}
	if result.DesiredAddress != 12 {
		t.Fatalf("result = %+v, want idempotent 12", result)
	}
}

func TestAssignmentsFromDetectors(t *testing.T) {
	detectors := []z21v1alpha1.CANDetector{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "trains", Name: "detector-a"},
			Spec:       z21v1alpha1.CANDetectorSpec{DesiredAddress: uint16Ptr(10)},
			Status:     z21v1alpha1.CANDetectorStatus{AssignedAddress: uint16Ptr(10)},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "trains", Name: "detector-b"},
			Spec:       z21v1alpha1.CANDetectorSpec{DesiredAddress: uint16Ptr(11)},
		},
	}

	assignments := assignmentsFromDetectors(detectors)
	if len(assignments) != 2 {
		t.Fatalf("assignments = %+v", assignments)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
