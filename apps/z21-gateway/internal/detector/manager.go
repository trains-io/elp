package detector

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
	z21client "github.com/trains-io/z21.go/client"
	"github.com/trains-io/z21.go/protocol"
)

const (
	discoveryCollectWindow = 2 * time.Second
	applyTimeout           = 5 * time.Second
)

// DiscoveredDetector is one CAN module seen during LAN_CAN_DETECTOR discovery.
type DiscoveredDetector struct {
	NetID  uint16
	Addr   uint16
	Ports  map[uint8]struct{}
}

// Manager creates and updates CANDetector CRs and applies module addresses on the Z21.
type Manager struct {
	client      client.Client
	namespace   string
	deviceName  string
	z21Host     string
	maintPort   int
}

// NewManager returns a detector manager for one Z21Device gateway.
func NewManager(cl client.Client, namespace, deviceName, z21Address string, maintPort int) (*Manager, error) {
	host, _, err := net.SplitHostPort(z21Address)
	if err != nil {
		return nil, fmt.Errorf("parse z21 address %q: %w", z21Address, err)
	}
	if maintPort <= 0 {
		maintPort = z21client.MaintenancePort
	}
	return &Manager{
		client:     cl,
		namespace:  namespace,
		deviceName: deviceName,
		z21Host:    host,
		maintPort:  maintPort,
	}, nil
}

// SyncDiscovery polls all CAN detectors and upserts CANDetector CRs with observed addresses.
func (m *Manager) SyncDiscovery(ctx context.Context, z21 *z21client.Client) error {
	msgs, err := z21.RequestCollect(ctx, protocol.GetAllCANDetectors(), discoveryCollectWindow)
	if err != nil {
		return fmt.Errorf("can discovery: %w", err)
	}
	return m.ProcessCANDetectorMessages(ctx, msgs)
}

// ProcessCANDetectorMessages upserts CANDetector CRs from LAN_CAN_DETECTOR replies or broadcasts.
func (m *Manager) ProcessCANDetectorMessages(ctx context.Context, msgs []protocol.Message) error {
	discovered, err := summarizeDiscovery(msgs)
	if err != nil {
		return err
	}

	for _, det := range discovered {
		if err := m.upsertDiscovered(ctx, det); err != nil {
			return err
		}
	}
	return nil
}

// ReconcileAddresses applies spec.desiredAddress when it differs from status.observedAddress.
func (m *Manager) ReconcileAddresses(ctx context.Context, z21 *z21client.Client) error {
	var list z21v1alpha1.CANDetectorList
	if err := m.client.List(ctx, &list, client.InNamespace(m.namespace)); err != nil {
		return err
	}

	for _, det := range list.Items {
		if det.Spec.DeviceRef.Name != m.deviceName {
			continue
		}
		if err := m.reconcileOne(ctx, z21, &det); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) reconcileOne(ctx context.Context, z21 *z21client.Client, det *z21v1alpha1.CANDetector) error {
	desired := det.Spec.DesiredAddress
	if desired == nil {
		return nil
	}
	if det.Status.ObservedAddress != nil && *det.Status.ObservedAddress == *desired {
		return m.markApplied(ctx, det)
	}

	maint, err := z21client.Dial(net.JoinHostPort(m.z21Host, strconv.Itoa(m.maintPort)))
	if err != nil {
		return fmt.Errorf("dial maintenance port: %w", err)
	}
	defer maint.Close()

	applyCtx, cancel := context.WithTimeout(ctx, applyTimeout)
	defer cancel()

	msg, err := protocol.SetCANDetectorModuleAddress(det.Spec.NetID, *desired, true)
	if err != nil {
		return err
	}
	if err := maint.Send(applyCtx, msg); err != nil {
		return fmt.Errorf("set module address netID=0x%04X addr=%d: %w", det.Spec.NetID, *desired, err)
	}

	observed, err := pollModuleAddress(applyCtx, z21, det.Spec.NetID)
	if err != nil {
		return err
	}

	patchBase := det.DeepCopy()
	det.Status.ObservedAddress = uint16Ptr(observed)
	if observed == *desired {
		det.Status.Phase = z21v1alpha1.CANDetectorPhaseApplied
		det.Status.Message = ""
	} else {
		det.Status.Phase = z21v1alpha1.CANDetectorPhaseAllocated
		det.Status.Message = fmt.Sprintf("module still reports address %d after apply", observed)
	}
	return m.client.Status().Patch(ctx, det, client.MergeFrom(patchBase))
}

func pollModuleAddress(ctx context.Context, z21 *z21client.Client, netID uint16) (uint16, error) {
	msgs, err := z21.RequestCollect(ctx, protocol.GetCANDetector(netID), discoveryCollectWindow)
	if err != nil {
		return 0, err
	}
	reports, err := protocol.CANDetectorReportsFromMessages(msgs)
	if err != nil {
		return 0, err
	}
	for _, report := range reports {
		if report.NetID == netID {
			return report.Addr, nil
		}
	}
	return 0, fmt.Errorf("no LAN_CAN_DETECTOR reply for netID 0x%04X", netID)
}

func (m *Manager) markApplied(ctx context.Context, det *z21v1alpha1.CANDetector) error {
	if det.Status.Phase == z21v1alpha1.CANDetectorPhaseApplied {
		return nil
	}
	patchBase := det.DeepCopy()
	det.Status.Phase = z21v1alpha1.CANDetectorPhaseApplied
	det.Status.Message = ""
	return m.client.Status().Patch(ctx, det, client.MergeFrom(patchBase))
}

func (m *Manager) upsertDiscovered(ctx context.Context, det DiscoveredDetector) error {
	name := detectorName(det.NetID)
	key := types.NamespacedName{Namespace: m.namespace, Name: name}

	current := &z21v1alpha1.CANDetector{}
	err := m.client.Get(ctx, key, current)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	adopt := true
	desired := &z21v1alpha1.CANDetector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: m.namespace,
			Labels: map[string]string{
				z21v1alpha1.LabelDeviceName: m.deviceName,
			},
		},
		Spec: z21v1alpha1.CANDetectorSpec{
			DeviceRef: corev1.LocalObjectReference{
				Name: m.deviceName,
			},
			NetID:         det.NetID,
			AdoptObserved: &adopt,
		},
	}

	if apierrors.IsNotFound(err) {
		if err := m.client.Create(ctx, desired); err != nil {
			return err
		}
		current = desired
	} else {
		patchBase := current.DeepCopy()
		current.Labels = desired.Labels
		current.Spec.NetID = det.NetID
		if current.Spec.DeviceRef.Name == "" {
			current.Spec.DeviceRef.Name = m.deviceName
		}
		if current.Spec.AdoptObserved == nil {
			current.Spec.AdoptObserved = &adopt
		}
		if err := m.client.Patch(ctx, current, client.MergeFrom(patchBase)); err != nil {
			return err
		}
	}

	statusBase := current.DeepCopy()
	addr := det.Addr
	if current.Status.ObservedAddress != nil && *current.Status.ObservedAddress == addr &&
		current.Status.Phase != "" {
		return nil
	}
	current.Status.ObservedAddress = &addr
	if current.Status.Phase == "" {
		current.Status.Phase = z21v1alpha1.CANDetectorPhasePending
	}
	return m.client.Status().Patch(ctx, current, client.MergeFrom(statusBase))
}

func summarizeDiscovery(msgs []protocol.Message) ([]DiscoveredDetector, error) {
	byNetID := map[uint16]*DiscoveredDetector{}

	for _, msg := range msgs {
		if msg.Header != protocol.HeaderLANCANDetector {
			continue
		}
		report, err := protocol.ParseCANDetector(msg.Data)
		if err != nil {
			return nil, err
		}
		if report.NetID == protocol.CANDetectorPollAll {
			continue
		}

		entry := byNetID[report.NetID]
		if entry == nil {
			entry = &DiscoveredDetector{NetID: report.NetID, Ports: map[uint8]struct{}{}}
			byNetID[report.NetID] = entry
		}
		if entry.Addr == 0 {
			entry.Addr = report.Addr
		}
		entry.Ports[report.Port] = struct{}{}
	}

	out := make([]DiscoveredDetector, 0, len(byNetID))
	for _, det := range byNetID {
		out = append(out, *det)
	}
	return out, nil
}

func detectorName(netID uint16) string {
	return fmt.Sprintf("detector-%04x", netID)
}

func uint16Ptr(v uint16) *uint16 {
	return &v
}

// ParseZ21Host returns the host part of a Z21 UDP address.
func ParseZ21Host(z21Address string) (string, error) {
	host, _, err := net.SplitHostPort(z21Address)
	if err != nil {
		return "", err
	}
	return host, nil
}

// NormalizeDetectorName validates a detector CR name for a netID.
func NormalizeDetectorName(name string) (uint16, bool) {
	const prefix = "detector-"
	if !strings.HasPrefix(name, prefix) {
		return 0, false
	}
	v, err := strconv.ParseUint(name[len(prefix):], 16, 16)
	if err != nil {
		return 0, false
	}
	return uint16(v), true
}
