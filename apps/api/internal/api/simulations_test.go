package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	simv1alpha1 "github.com/trains-io/elp/operators/z21-sim-controller/api/v1alpha1"
	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

func testSimulatorDeviceCR() *z21v1alpha1.Z21Device {
	return &z21v1alpha1.Z21Device{
		ObjectMeta: metav1.ObjectMeta{Name: "lab", Namespace: "default"},
		Spec: z21v1alpha1.Z21DeviceSpec{
			Backend: z21v1alpha1.BackendSpec{Type: z21v1alpha1.BackendSimulator},
			NATS:    z21v1alpha1.NatsSpec{URL: "nats://nats:4222"},
		},
	}
}

func newFakeAPIClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := z21v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := simv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func TestSimulateCANCreatesSimulation(t *testing.T) {
	h := &DeviceHandler{
		Client:  newFakeAPIClient(t, testSimulatorDeviceCR()),
		Control: noopControl{},
	}

	body := `{
		"canDetectors": [{ "netID": 56068, "name": "occupancy-a" }]
	}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", "default")
	rctx.URLParams.Add("name", "lab")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.simulateCAN(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}

	var got Simulation
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "lab" || got.DeviceRef != "lab" || len(got.CANDetectors) != 1 {
		t.Fatalf("simulation = %#v", got)
	}

	stored := &simv1alpha1.Simulation{}
	if err := h.Client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "lab"}, stored); err != nil {
		t.Fatal(err)
	}
	if stored.Spec.CANDetectors[0].NetID != 0xDB04 || stored.Spec.CANDetectors[0].PortCount != 8 {
		t.Fatalf("stored detectors = %#v", stored.Spec.CANDetectors)
	}
}

func TestSimulateCANMergesDetectors(t *testing.T) {
	existing := &simv1alpha1.Simulation{
		ObjectMeta: metav1.ObjectMeta{Name: "lab", Namespace: "default"},
		Spec: simv1alpha1.SimulationSpec{
			DeviceRef: corev1.LocalObjectReference{Name: "lab"},
			CANDetectors: []simv1alpha1.SimulationCANDetector{{
				NetID: 0xDB04, ModuleAddress: 1, PortCount: 8,
			}},
		},
	}
	h := &DeviceHandler{
		Client:  newFakeAPIClient(t, testSimulatorDeviceCR(), existing),
		Control: noopControl{},
	}

	body := `{
		"canDetectors": [{ "netID": 56069 }]
	}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", "default")
	rctx.URLParams.Add("name", "lab")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.simulateCAN(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}

	stored := &simv1alpha1.Simulation{}
	if err := h.Client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "lab"}, stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.Spec.CANDetectors) != 2 {
		t.Fatalf("detectors = %#v", stored.Spec.CANDetectors)
	}
}

func TestSimulateCANRejectsHardwareDevice(t *testing.T) {
	h := &DeviceHandler{
		Client:  newFakeAPIClient(t, testDeviceCR()),
		Control: noopControl{},
	}

	body := `{"canDetectors":[{"netID":56068}]}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", "default")
	rctx.URLParams.Add("name", "basement")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.simulateCAN(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
}

func TestSimulateCANThroughServerRouting(t *testing.T) {
	cl := newFakeAPIClient(t, testSimulatorDeviceCR())
	srv := NewServer(cl, nil, noopControl{})

	body := `{"canDetectors":[{"netID":56068}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/default/devices/lab/simulate/can", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
}
