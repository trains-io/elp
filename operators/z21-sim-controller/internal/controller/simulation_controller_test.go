package controller

import (
	"context"
	"testing"

	controlv1 "github.com/trains-io/elp/apps/z21-sim/api/control/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	simv1alpha1 "github.com/trains-io/elp/operators/z21-sim-controller/api/v1alpha1"
	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

type recordingSimClient struct {
	requests []*controlv1.NotifyCANDeviceDetectedRequest
	err      error
}

func (c *recordingSimClient) NotifyCANDeviceDetected(_ context.Context, req *controlv1.NotifyCANDeviceDetectedRequest) error {
	c.requests = append(c.requests, req)
	return c.err
}

type staticClientProvider struct {
	client SimulatorClient
}

func (p staticClientProvider) ClientFor(_ string) SimulatorClient {
	return p.client
}

func TestSimulationReconcilerAppliesCANDetectors(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := z21v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := simv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	device := &z21v1alpha1.Z21Device{
		ObjectMeta: metav1.ObjectMeta{Name: "lab", Namespace: "default"},
		Spec: z21v1alpha1.Z21DeviceSpec{
			Backend: z21v1alpha1.BackendSpec{Type: z21v1alpha1.BackendSimulator},
		},
	}
	simulation := &simv1alpha1.Simulation{
		ObjectMeta: metav1.ObjectMeta{Name: "layout", Namespace: "default"},
		Spec: simv1alpha1.SimulationSpec{
			DeviceRef: corev1.LocalObjectReference{Name: "lab"},
			CANDetectors: []simv1alpha1.SimulationCANDetector{{
				NetID: 0xDB04,
				Name:  "detector-a",
			}},
			CANBoosters: []simv1alpha1.SimulationCANBooster{{
				NetID: 0xBEEF,
				Name:  "booster-a",
			}},
		},
	}

	simClient := &recordingSimClient{}
	r := &SimulationReconciler{
		Client:            fake.NewClientBuilder().WithScheme(scheme).WithObjects(device, simulation).WithStatusSubresource(simulation).Build(),
		Scheme:            scheme,
		WorkloadNamespace: "elp",
		SimClients:        staticClientProvider{client: simClient},
	}

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "layout"},
	}); err != nil {
		t.Fatal(err)
	}

	if len(simClient.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(simClient.requests))
	}
	if simClient.requests[0].Kind != controlv1.CANDeviceKind_CAN_DEVICE_KIND_DETECTOR {
		t.Fatalf("detector kind = %v", simClient.requests[0].Kind)
	}
	if simClient.requests[0].ModuleAddr != 1 || simClient.requests[0].PortCount != 8 {
		t.Fatalf("detector defaults = module %d ports %d", simClient.requests[0].ModuleAddr, simClient.requests[0].PortCount)
	}
	if simClient.requests[1].Kind != controlv1.CANDeviceKind_CAN_DEVICE_KIND_BOOSTER {
		t.Fatalf("booster kind = %v", simClient.requests[1].Kind)
	}

	latest := &simv1alpha1.Simulation{}
	if err := r.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "layout"}, latest); err != nil {
		t.Fatal(err)
	}
	if latest.Status.Phase != simv1alpha1.SimulationPhaseApplied {
		t.Fatalf("phase = %q", latest.Status.Phase)
	}
	if latest.Status.AppliedCANDevices != 2 {
		t.Fatalf("applied = %d", latest.Status.AppliedCANDevices)
	}
}

func TestSimulationReconcilerFailsMissingDevice(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := simv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := z21v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	simulation := &simv1alpha1.Simulation{
		ObjectMeta: metav1.ObjectMeta{Name: "layout", Namespace: "default"},
		Spec: simv1alpha1.SimulationSpec{
			DeviceRef: corev1.LocalObjectReference{Name: "missing"},
		},
	}

	simClient := &recordingSimClient{}
	r := &SimulationReconciler{
		Client:     fake.NewClientBuilder().WithScheme(scheme).WithObjects(simulation).WithStatusSubresource(simulation).Build(),
		Scheme:     scheme,
		SimClients: staticClientProvider{client: simClient},
	}

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "layout"},
	}); err != nil {
		t.Fatal(err)
	}
	if len(simClient.requests) != 0 {
		t.Fatalf("expected no RPCs, got %d", len(simClient.requests))
	}

	latest := &simv1alpha1.Simulation{}
	if err := r.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "layout"}, latest); err != nil {
		t.Fatal(err)
	}
	if latest.Status.Phase != simv1alpha1.SimulationPhaseFailed {
		t.Fatalf("phase = %q", latest.Status.Phase)
	}
}
