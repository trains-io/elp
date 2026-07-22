package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	simv1alpha1 "github.com/trains-io/elp/operators/z21-sim-controller/api/v1alpha1"
	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

const (
	defaultDetectorModuleAddress uint16 = 1
	defaultDetectorPortCount     uint16 = 8
)

func (h *DeviceHandler) simulateCAN(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	deviceName := chi.URLParam(r, "name")

	var req SimulateCANRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.CANDetectors) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("canDetectors is required"))
		return
	}

	device := &z21v1alpha1.Z21Device{}
	if err := h.Client.Get(r.Context(), types.NamespacedName{Namespace: namespace, Name: deviceName}, device); err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, fmt.Errorf("device %q not found", deviceName))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if device.Spec.Backend.Type != z21v1alpha1.BackendSimulator {
		writeError(w, http.StatusBadRequest, fmt.Errorf("device %q backend is %q, expected %q", deviceName, device.Spec.Backend.Type, z21v1alpha1.BackendSimulator))
		return
	}

	simulationName := req.SimulationName
	if simulationName == "" {
		simulationName = deviceName
	}

	incoming, err := detectorsFromAPI(req.CANDetectors)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	simulation := &simv1alpha1.Simulation{}
	err = h.Client.Get(r.Context(), types.NamespacedName{Namespace: namespace, Name: simulationName}, simulation)
	switch {
	case apierrors.IsNotFound(err):
		simulation = &simv1alpha1.Simulation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      simulationName,
				Namespace: namespace,
			},
			Spec: simv1alpha1.SimulationSpec{
				DeviceRef:    corev1.LocalObjectReference{Name: deviceName},
				CANDetectors: incoming,
			},
		}
		if err := h.Client.Create(r.Context(), simulation); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	case err != nil:
		writeError(w, http.StatusInternalServerError, err)
		return
	default:
		if simulation.Spec.DeviceRef.Name != deviceName {
			writeError(w, http.StatusConflict, fmt.Errorf("simulation %q already exists for device %q", simulationName, simulation.Spec.DeviceRef.Name))
			return
		}
		patchBase := simulation.DeepCopy()
		simulation.Spec.CANDetectors = mergeCANDetectors(simulation.Spec.CANDetectors, incoming)
		if err := h.Client.Patch(r.Context(), simulation, client.MergeFrom(patchBase)); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	writeJSON(w, http.StatusOK, simulationFromCR(simulation))
}

func detectorsFromAPI(detectors []SimulationCANDetector) ([]simv1alpha1.SimulationCANDetector, error) {
	out := make([]simv1alpha1.SimulationCANDetector, 0, len(detectors))
	for _, detector := range detectors {
		if detector.NetID == 0 {
			return nil, fmt.Errorf("canDetectors[].netID is required")
		}
		moduleAddr := detector.ModuleAddress
		if moduleAddr == 0 {
			moduleAddr = defaultDetectorModuleAddress
		}
		portCount := detector.PortCount
		if portCount == 0 {
			portCount = defaultDetectorPortCount
		}
		out = append(out, simv1alpha1.SimulationCANDetector{
			NetID:         detector.NetID,
			Name:          detector.Name,
			ModuleAddress: moduleAddr,
			PortCount:     portCount,
		})
	}
	return out, nil
}

func mergeCANDetectors(existing, incoming []simv1alpha1.SimulationCANDetector) []simv1alpha1.SimulationCANDetector {
	byNetID := make(map[uint16]simv1alpha1.SimulationCANDetector, len(existing)+len(incoming))
	for _, detector := range existing {
		byNetID[detector.NetID] = detector
	}
	for _, detector := range incoming {
		byNetID[detector.NetID] = detector
	}
	netIDs := make([]uint16, 0, len(byNetID))
	for netID := range byNetID {
		netIDs = append(netIDs, netID)
	}
	sort.Slice(netIDs, func(i, j int) bool { return netIDs[i] < netIDs[j] })

	out := make([]simv1alpha1.SimulationCANDetector, 0, len(netIDs))
	for _, netID := range netIDs {
		out = append(out, byNetID[netID])
	}
	return out
}

func simulationFromCR(cr *simv1alpha1.Simulation) Simulation {
	out := Simulation{
		Name:      cr.Name,
		Namespace: cr.Namespace,
		DeviceRef: cr.Spec.DeviceRef.Name,
	}
	if len(cr.Spec.CANDetectors) > 0 {
		out.CANDetectors = make([]SimulationCANDetector, 0, len(cr.Spec.CANDetectors))
		for _, detector := range cr.Spec.CANDetectors {
			out.CANDetectors = append(out.CANDetectors, SimulationCANDetector{
				NetID:         detector.NetID,
				Name:          detector.Name,
				ModuleAddress: detector.ModuleAddress,
				PortCount:     detector.PortCount,
			})
		}
	}
	if cr.Status.Phase != "" || cr.Status.Message != "" || cr.Status.AppliedCANDevices != 0 {
		out.Status = &SimulationStatus{
			Phase:             string(cr.Status.Phase),
			Message:           cr.Status.Message,
			AppliedCANDevices: cr.Status.AppliedCANDevices,
		}
	}
	return out
}
