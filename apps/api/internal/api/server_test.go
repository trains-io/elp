package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/apimachinery/pkg/types"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

func TestCreateDeviceThroughServerRouting(t *testing.T) {
	cl := newFakeDeviceClient(t)
	srv := NewServer(cl, nil, noopControl{})

	name := "routing-test"
	body := `{
		"backend": { "type": "simulator" },
		"nats": { "url": "nats://nats.default.svc.cluster.local.:4222" }
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/default/devices/"+name, bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}

	stored := &z21v1alpha1.Z21Device{}
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: name}, stored); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.Name != name {
		t.Fatalf("stored name = %q", stored.Name)
	}
}
