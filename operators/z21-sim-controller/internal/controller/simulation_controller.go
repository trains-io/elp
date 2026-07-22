package controller

import (
	"context"
	"fmt"

	controlv1 "github.com/trains-io/elp/apps/z21-sim/api/control/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	simv1alpha1 "github.com/trains-io/elp/operators/z21-sim-controller/api/v1alpha1"
	"github.com/trains-io/elp/operators/z21-sim-controller/internal/simclient"
	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

const (
	defaultDetectorModuleAddress uint16 = 1
	defaultDetectorPortCount     uint16 = 8
	defaultBoosterOutputPort     uint16 = 1
)

// SimulatorClient applies control-plane RPCs to z21-sim.
type SimulatorClient interface {
	NotifyCANDeviceDetected(ctx context.Context, req *controlv1.NotifyCANDeviceDetectedRequest) error
}

// ClientProvider resolves a simulator client for a control endpoint.
type ClientProvider interface {
	ClientFor(addr string) SimulatorClient
}

type clientPool struct {
	pool simclient.Pool
}

func (p clientPool) ClientFor(addr string) SimulatorClient {
	return p.pool.ClientFor(addr)
}

// SimulationReconciler drives z21-sim from Simulation CRs via cluster DNS.
type SimulationReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	WorkloadNamespace string
	SimClients        ClientProvider
}

// +kubebuilder:rbac:groups=z21.trains.io,resources=simulations,verbs=get;list;watch
// +kubebuilder:rbac:groups=z21.trains.io,resources=simulations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=z21.trains.io,resources=z21devices,verbs=get;list;watch

func (r *SimulationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var simulation simv1alpha1.Simulation
	if err := r.Get(ctx, req.NamespacedName, &simulation); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if simulation.Spec.DeviceRef.Name == "" {
		return r.patchStatus(ctx, &simulation, simv1alpha1.SimulationPhaseFailed, "spec.deviceRef.name is required", 0)
	}

	var device z21v1alpha1.Z21Device
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: simulation.Namespace,
		Name:      simulation.Spec.DeviceRef.Name,
	}, &device); err != nil {
		if apierrors.IsNotFound(err) {
			return r.patchStatus(ctx, &simulation, simv1alpha1.SimulationPhaseFailed,
				fmt.Sprintf("Z21Device %q not found in namespace %q", simulation.Spec.DeviceRef.Name, simulation.Namespace), 0)
		}
		return ctrl.Result{}, err
	}
	if device.Spec.Backend.Type != z21v1alpha1.BackendSimulator {
		return r.patchStatus(ctx, &simulation, simv1alpha1.SimulationPhaseFailed,
			fmt.Sprintf("Z21Device %q backend is %q, expected %q", device.Name, device.Spec.Backend.Type, z21v1alpha1.BackendSimulator), 0)
	}

	if r.SimClients == nil {
		return r.patchStatus(ctx, &simulation, simv1alpha1.SimulationPhaseFailed, "simulator gRPC client pool is not configured", 0)
	}

	grpcAddr := z21v1alpha1.SimulatorGRPCAddress(r.workloadNamespace(), device.Name)
	simClient := r.SimClients.ClientFor(grpcAddr)

	applied, err := r.applyCANDevices(ctx, simClient, &simulation)
	if err != nil {
		return r.patchStatus(ctx, &simulation, simv1alpha1.SimulationPhaseFailed, err.Error(), applied)
	}

	logger.Info("applied simulation", "simulation", simulation.Name, "device", device.Name, "endpoint", grpcAddr, "canDevices", applied)
	return r.patchStatus(ctx, &simulation, simv1alpha1.SimulationPhaseApplied, "", applied)
}

func (r *SimulationReconciler) workloadNamespace() string {
	if r.WorkloadNamespace != "" {
		return r.WorkloadNamespace
	}
	return z21v1alpha1.DefaultWorkloadNamespace
}

func (r *SimulationReconciler) applyCANDevices(ctx context.Context, simClient SimulatorClient, simulation *simv1alpha1.Simulation) (int32, error) {
	var applied int32

	for _, detector := range simulation.Spec.CANDetectors {
		moduleAddr := detector.ModuleAddress
		if moduleAddr == 0 {
			moduleAddr = defaultDetectorModuleAddress
		}
		portCount := detector.PortCount
		if portCount == 0 {
			portCount = defaultDetectorPortCount
		}
		req := &controlv1.NotifyCANDeviceDetectedRequest{
			NetId:      uint32(detector.NetID),
			Kind:       controlv1.CANDeviceKind_CAN_DEVICE_KIND_DETECTOR,
			Name:       detector.Name,
			ModuleAddr: uint32(moduleAddr),
			PortCount:  uint32(portCount),
		}
		if err := simClient.NotifyCANDeviceDetected(ctx, req); err != nil {
			return applied, fmt.Errorf("register CAN detector netID=0x%04X: %w", detector.NetID, err)
		}
		applied++
	}

	for _, booster := range simulation.Spec.CANBoosters {
		outputPort := booster.OutputPort
		if outputPort == 0 {
			outputPort = defaultBoosterOutputPort
		}
		req := &controlv1.NotifyCANDeviceDetectedRequest{
			NetId:      uint32(booster.NetID),
			Kind:       controlv1.CANDeviceKind_CAN_DEVICE_KIND_BOOSTER,
			Name:       booster.Name,
			OutputPort: uint32(outputPort),
		}
		if err := simClient.NotifyCANDeviceDetected(ctx, req); err != nil {
			return applied, fmt.Errorf("register CAN booster netID=0x%04X: %w", booster.NetID, err)
		}
		applied++
	}

	return applied, nil
}

func (r *SimulationReconciler) patchStatus(
	ctx context.Context,
	simulation *simv1alpha1.Simulation,
	phase simv1alpha1.SimulationPhase,
	message string,
	applied int32,
) (ctrl.Result, error) {
	orig := simulation.Status.DeepCopy()
	simulation.Status.Phase = phase
	simulation.Status.Message = message
	simulation.Status.AppliedCANDevices = applied
	simulation.Status.ObservedGeneration = simulation.Generation
	if equality.Semantic.DeepEqual(orig, &simulation.Status) {
		return ctrl.Result{}, nil
	}
	if err := r.Status().Update(ctx, simulation); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *SimulationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&simv1alpha1.Simulation{}).
		Complete(r)
}

// NewClientPoolProvider wraps a simclient.Pool for the reconciler.
func NewClientPoolProvider(pool simclient.Pool) ClientProvider {
	return clientPool{pool: pool}
}

// Ensure SimulationReconciler uses simclient in production builds.
var _ SimulatorClient = (*simclient.GRPCClient)(nil)
