package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

type recordingControl struct {
	calls int
}

func (r *recordingControl) PublishSetBroadcastFlags(context.Context, *z21v1alpha1.Z21Device) error {
	r.calls++
	return nil
}

type noopControl struct{}

func (noopControl) PublishSetBroadcastFlags(context.Context, *z21v1alpha1.Z21Device) error {
	return nil
}

func testDeviceCR() *z21v1alpha1.Z21Device {
	flags := z21v1alpha1.DefaultBroadcastFlags
	return &z21v1alpha1.Z21Device{
		ObjectMeta: metav1.ObjectMeta{Name: "basement", Namespace: "default"},
		Spec: z21v1alpha1.Z21DeviceSpec{
			Backend: z21v1alpha1.BackendSpec{
				Type:     z21v1alpha1.BackendHardware,
				Hardware: &z21v1alpha1.HardwareBackend{Host: "127.0.0.1"},
			},
			NATS:           z21v1alpha1.NatsSpec{URL: "nats://nats:4222"},
			BroadcastFlags: &flags,
		},
	}
}

func newFakeDeviceClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := z21v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func TestListDevices(t *testing.T) {
	cr := testDeviceCR()
	h := &DeviceHandler{
		Client:  newFakeDeviceClient(t, cr),
		Control: noopControl{},
	}

	list, err := h.List(context.Background(), "default")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].Name != "basement" {
		t.Fatalf("List() = %#v", list)
	}
}

func TestCreateDeviceHTTP(t *testing.T) {
	h := &DeviceHandler{
		Client:  newFakeDeviceClient(t),
		Control: noopControl{},
	}

	body := `{
		"name": "basement",
		"address": "192.168.0.42:21105",
		"nats": { "url": "nats://nats.default.svc:4222" }
	}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	req = req.WithContext(context.Background())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", "default")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.createDevice(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}

	var got Device
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "basement" || got.Address != "192.168.0.42:21105" {
		t.Fatalf("device = %#v", got)
	}
}

func TestCreateDeviceSimulatorHTTP(t *testing.T) {
	h := &DeviceHandler{
		Client:  newFakeDeviceClient(t),
		Control: noopControl{},
	}

	body := `{
		"name": "sim-bench",
		"backend": { "type": "simulator" },
		"nats": { "url": "nats://nats.default.svc:4222" }
	}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	req = req.WithContext(context.Background())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", "default")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.createDevice(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}

	var got Device
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "sim-bench" || got.Address != "z21-sim-sim-bench.elp.svc.cluster.local.:21105" {
		t.Fatalf("device = %#v", got)
	}
}

func TestCreateDeviceSimulatorByURLHTTP(t *testing.T) {
	cl := newFakeDeviceClient(t)
	h := &DeviceHandler{
		Client:  cl,
		Control: noopControl{},
	}

	body := `{
		"backend": { "type": "simulator" },
		"nats": { "url": "nats://nats.default.svc:4222" }
	}`
	req := httptest.NewRequest(http.MethodPost, "/sim-bench", bytes.NewBufferString(body))
	req = req.WithContext(context.Background())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", "default")
	rctx.URLParams.Add("name", "sim-bench")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.createDevice(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}

	stored := &z21v1alpha1.Z21Device{}
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "sim-bench"}, stored); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.Spec.Backend.Type != z21v1alpha1.BackendSimulator {
		t.Fatalf("backend = %#v", stored.Spec.Backend)
	}
}

func TestCreateDeviceSimulatorPayloadIncludesName(t *testing.T) {
	data, err := json.Marshal(DeviceCreate{
		Name: "dev-01",
		Backend: &BackendCreate{
			Type: "simulator",
		},
		NATS: NatsConfig{URL: "nats://nats:4222"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var req DeviceCreate
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatal(err)
	}
	if req.Name != "dev-01" {
		t.Fatalf("name = %q", req.Name)
	}
}

func TestPatchBroadcastFlagsPublishesControl(t *testing.T) {
	cr := testDeviceCR()
	control := &recordingControl{}
	h := &DeviceHandler{
		Client:  newFakeDeviceClient(t, cr),
		Control: control,
	}

	newFlags := uint32(0x103)
	body, err := json.Marshal(DeviceUpdate{BroadcastFlags: &newFlags})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/basement", bytes.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", "default")
	rctx.URLParams.Add("name", "basement")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.patchDevice(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	if control.calls != 1 {
		t.Fatalf("PublishSetBroadcastFlags calls = %d, want 1", control.calls)
	}
}
