package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

type DeviceHandler struct {
	Client    client.Client
	StreamHub *StreamHub
	Control   DeviceControlPublisher
}

// DeviceControlPublisher pushes live commands to a device gateway.
type DeviceControlPublisher interface {
	PublishSetBroadcastFlags(ctx context.Context, device *z21v1alpha1.Z21Device) error
}

func (h *DeviceHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/stream", h.streamDevices)
	r.Get("/", h.listDevices)
	r.Post("/", h.createDevice)
	r.Get("/{name}", h.getDevice)
	r.Patch("/{name}", h.patchDevice)
	r.Delete("/{name}", h.deleteDevice)
	return r
}

func (h *DeviceHandler) listDevices(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")

	var devices z21v1alpha1.Z21DeviceList
	if err := h.Client.List(r.Context(), &devices, client.InNamespace(namespace)); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	items := make([]Device, 0, len(devices.Items))
	for i := range devices.Items {
		items = append(items, deviceFromCR(&devices.Items[i]))
	}
	writeJSON(w, http.StatusOK, DeviceList{Items: items})
}

func (h *DeviceHandler) createDevice(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")

	var req DeviceCreate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := validateDeviceCreate(req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	spec, err := specFromCreate(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	device := &z21v1alpha1.Z21Device{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: namespace,
		},
		Spec: spec,
	}

	if err := h.Client.Create(r.Context(), device); err != nil {
		if apierrors.IsAlreadyExists(err) {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, deviceFromCR(device))
}

func (h *DeviceHandler) getDevice(w http.ResponseWriter, r *http.Request) {
	device, ok := h.fetchDevice(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, deviceFromCR(device))
}

func (h *DeviceHandler) patchDevice(w http.ResponseWriter, r *http.Request) {
	device, ok := h.fetchDevice(w, r)
	if !ok {
		return
	}

	var req DeviceUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	patchBase := device.DeepCopy()
	broadcastFlagsChanged := req.BroadcastFlags != nil
	if err := applyUpdate(&device.Spec, req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.Client.Patch(r.Context(), device, client.MergeFrom(patchBase)); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if broadcastFlagsChanged {
		if err := h.Control.PublishSetBroadcastFlags(r.Context(), device); err != nil {
			writeError(w, http.StatusBadGateway, fmt.Errorf("publish broadcast flags: %w", err))
			return
		}
	}

	writeJSON(w, http.StatusOK, deviceFromCR(device))
}

func (h *DeviceHandler) deleteDevice(w http.ResponseWriter, r *http.Request) {
	device, ok := h.fetchDevice(w, r)
	if !ok {
		return
	}

	if err := h.Client.Delete(r.Context(), device); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DeviceHandler) fetchDevice(w http.ResponseWriter, r *http.Request) (*z21v1alpha1.Z21Device, bool) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	device := &z21v1alpha1.Z21Device{}
	if err := h.Client.Get(r.Context(), types.NamespacedName{Namespace: namespace, Name: name}, device); err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, err)
			return nil, false
		}
		writeError(w, http.StatusInternalServerError, err)
		return nil, false
	}
	return device, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, ErrorResponse{Error: err.Error()})
}

// List is exposed for tests with a custom context.
func (h *DeviceHandler) List(ctx context.Context, namespace string) (DeviceList, error) {
	var devices z21v1alpha1.Z21DeviceList
	if err := h.Client.List(ctx, &devices, client.InNamespace(namespace)); err != nil {
		return DeviceList{}, err
	}
	items := make([]Device, 0, len(devices.Items))
	for i := range devices.Items {
		items = append(items, deviceFromCR(&devices.Items[i]))
	}
	return DeviceList{Items: items}, nil
}
