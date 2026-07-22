package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSimulateCANAppliesSimulation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/namespaces/default/devices/lab/simulate/can" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		detectors, ok := body["canDetectors"].([]any)
		if !ok || len(detectors) != 1 {
			t.Fatalf("body = %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":      "lab",
			"namespace": "default",
			"deviceRef": "lab",
			"canDetectors": []map[string]any{{
				"netID": float64(56068),
				"name":  "occupancy-a",
			}},
			"status": map[string]any{
				"phase":             "Pending",
				"appliedCANDevices": float64(0),
			},
		})
	}))
	defer srv.Close()

	stdout, _ := captureOutput(t, func() {
		rootCmd.SetArgs([]string{
			"simulate", "can", "lab",
			"--server", srv.URL,
			"--namespace", "default",
		})
		if err := rootCmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(stdout, `simulation "lab" applied`) {
		t.Fatalf("stdout = %q", stdout)
	}
}
