package controller

import (
	"context"
	"fmt"

	"github.com/trains-io/elp/packages/canpool"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

// CANDetectorReconciler assigns module addresses from a CANAddressPool referenced by Z21Device.
type CANDetectorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=z21.trains.io,resources=candetectors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=z21.trains.io,resources=candetectors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=z21.trains.io,resources=canaddresspools,verbs=get;list;watch
// +kubebuilder:rbac:groups=z21.trains.io,resources=canaddresspools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=z21.trains.io,resources=z21devices,verbs=get;list;watch

func (r *CANDetectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var detector z21v1alpha1.CANDetector
	if err := r.Get(ctx, req.NamespacedName, &detector); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	deviceName := detector.Spec.DeviceRef.Name
	if deviceName == "" {
		return r.patchFailed(ctx, &detector, "spec.deviceRef.name is required")
	}

	var device z21v1alpha1.Z21Device
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: deviceName}, &device); err != nil {
		if apierrors.IsNotFound(err) {
			return r.patchFailed(ctx, &detector, fmt.Sprintf("Z21Device %q not found", deviceName))
		}
		return ctrl.Result{}, err
	}

	poolRef := device.Spec.CANAddressPoolRef
	if poolRef == nil || poolRef.Name == "" {
		return r.patchFailed(ctx, &detector, fmt.Sprintf("Z21Device %q has no spec.canAddressPoolRef", deviceName))
	}

	var poolCR z21v1alpha1.CANAddressPool
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: poolRef.Name}, &poolCR); err != nil {
		if apierrors.IsNotFound(err) {
			return r.patchFailed(ctx, &detector, fmt.Sprintf("CANAddressPool %q not found", poolRef.Name))
		}
		return ctrl.Result{}, err
	}

	detectors, err := r.listDeviceDetectors(ctx, req.Namespace, deviceName)
	if err != nil {
		return ctrl.Result{}, err
	}

	result, err := allocateDetectorAddress(poolConfigFromCR(&poolCR), detectors, detector)
	if err != nil {
		return ctrl.Result{}, err
	}

	orig := detector.DeepCopy()
	labelChanged := ensureDeviceLabel(&detector)

	if result.Phase == z21v1alpha1.CANDetectorPhaseAllocated {
		detector.Spec.DesiredAddress = uint16Ptr(result.DesiredAddress)
		detector.Status.AssignedAddress = uint16Ptr(result.AssignedAddress)
		detector.Status.Phase = result.Phase
		detector.Status.Message = ""
	} else {
		detector.Status.Phase = result.Phase
		detector.Status.Message = result.Message
	}

	specChanged := !equality.Semantic.DeepEqual(orig.Spec, detector.Spec) || labelChanged
	statusChanged := !equality.Semantic.DeepEqual(orig.Status, detector.Status)

	if specChanged {
		if err := r.Update(ctx, &detector); err != nil {
			return ctrl.Result{}, err
		}
	}
	if statusChanged {
		if err := r.Status().Update(ctx, &detector); err != nil {
			return ctrl.Result{}, err
		}
	}

	if result.Phase == z21v1alpha1.CANDetectorPhaseAllocated {
		if err := r.updatePoolStatus(ctx, &poolCR, result.Stats); err != nil {
			logger.Error(err, "failed to update CANAddressPool status", "pool", poolRef.Name)
		}
	}

	if specChanged || statusChanged {
		logger.Info("reconciled CANDetector",
			"netID", formatNetID(detector.Spec.NetID),
			"phase", detector.Status.Phase,
			"desired", detector.Spec.DesiredAddress,
		)
	}
	return ctrl.Result{}, nil
}

func (r *CANDetectorReconciler) patchFailed(ctx context.Context, detector *z21v1alpha1.CANDetector, message string) (ctrl.Result, error) {
	orig := detector.Status
	detector.Status.Phase = z21v1alpha1.CANDetectorPhaseFailed
	detector.Status.Message = message
	if equality.Semantic.DeepEqual(orig, detector.Status) {
		return ctrl.Result{}, nil
	}
	if err := r.Status().Update(ctx, detector); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *CANDetectorReconciler) listDeviceDetectors(ctx context.Context, namespace, deviceName string) ([]z21v1alpha1.CANDetector, error) {
	var list z21v1alpha1.CANDetectorList
	if err := r.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	out := make([]z21v1alpha1.CANDetector, 0)
	for _, det := range list.Items {
		if det.Spec.DeviceRef.Name == deviceName {
			out = append(out, det)
		}
	}
	return out, nil
}

func (r *CANDetectorReconciler) updatePoolStatus(ctx context.Context, pool *z21v1alpha1.CANAddressPool, stats canpool.Stats) error {
	want := poolStatusFromStats(stats)
	if equality.Semantic.DeepEqual(pool.Status, want) {
		return nil
	}
	patch := pool.DeepCopy()
	patch.Status = want
	return r.Status().Update(ctx, patch)
}

func (r *CANDetectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&z21v1alpha1.CANDetector{}).
		Watches(
			&z21v1alpha1.Z21Device{},
			handler.EnqueueRequestsFromMapFunc(r.mapDeviceToDetectors),
		).
		Watches(
			&z21v1alpha1.CANAddressPool{},
			handler.EnqueueRequestsFromMapFunc(r.mapPoolToDetectors),
		).
		Complete(r)
}

func (r *CANDetectorReconciler) mapDeviceToDetectors(ctx context.Context, obj client.Object) []reconcile.Request {
	device, ok := obj.(*z21v1alpha1.Z21Device)
	if !ok {
		return nil
	}
	return r.enqueueDetectorsForDevice(ctx, device.Namespace, device.Name)
}

func (r *CANDetectorReconciler) mapPoolToDetectors(ctx context.Context, obj client.Object) []reconcile.Request {
	pool, ok := obj.(*z21v1alpha1.CANAddressPool)
	if !ok {
		return nil
	}

	var devices z21v1alpha1.Z21DeviceList
	if err := r.List(ctx, &devices, client.InNamespace(pool.Namespace)); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, device := range devices.Items {
		ref := device.Spec.CANAddressPoolRef
		if ref == nil || ref.Name != pool.Name {
			continue
		}
		requests = append(requests, r.enqueueDetectorsForDevice(ctx, device.Namespace, device.Name)...)
	}
	return requests
}

func (r *CANDetectorReconciler) enqueueDetectorsForDevice(ctx context.Context, namespace, deviceName string) []reconcile.Request {
	detectors, err := r.listDeviceDetectors(ctx, namespace, deviceName)
	if err != nil {
		return nil
	}
	requests := make([]reconcile.Request, 0, len(detectors))
	for _, det := range detectors {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: namespace, Name: det.Name},
		})
	}
	return requests
}
