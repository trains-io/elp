package controller

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

func TestDesiredGatewayDeployment(t *testing.T) {
	device := &z21v1alpha1.Z21Device{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "basement",
			Namespace: "trains",
		},
		Spec: z21v1alpha1.Z21DeviceSpec{
			Backend: z21v1alpha1.BackendSpec{
				Type: z21v1alpha1.BackendHardware,
				Hardware: &z21v1alpha1.HardwareBackend{
					Host: "192.168.0.42",
				},
			},
			NATS: z21v1alpha1.NatsSpec{
				URL: "nats://nats.nats.svc:4222",
			},
			Gateway: z21v1alpha1.GatewaySpec{
				HostNetwork: true,
				NodeSelector: map[string]string{
					"trains.io/edge": "true",
				},
			},
		},
	}

	z21Addr, err := device.Z21Address()
	if err != nil {
		t.Fatal(err)
	}

	dep := desiredGatewayDeployment(device, "gateway:test", "z21-gateway-basement", z21Addr)
	dep.Name = gatewayDeploymentName(device)

	if dep.Name != "z21-gateway-basement" {
		t.Fatalf("name = %q", dep.Name)
	}
	if *dep.Spec.Replicas != 1 {
		t.Fatalf("replicas = %d", *dep.Spec.Replicas)
	}
	if !dep.Spec.Template.Spec.HostNetwork {
		t.Fatal("expected hostNetwork")
	}
	if dep.Spec.Template.Spec.DNSPolicy != corev1.DNSClusterFirstWithHostNet {
		t.Fatalf("dnsPolicy = %q, want ClusterFirstWithHostNet", dep.Spec.Template.Spec.DNSPolicy)
	}
	if dep.Spec.Template.Spec.NodeSelector["trains.io/edge"] != "true" {
		t.Fatal("expected node selector")
	}
	if dep.Spec.Template.Spec.ServiceAccountName != "z21-gateway-basement" {
		t.Fatalf("serviceAccountName = %q", dep.Spec.Template.Spec.ServiceAccountName)
	}

	env := dep.Spec.Template.Spec.Containers[0].Env
	if envValue(env, "Z21_ADDRESS") != "192.168.0.42:21105" {
		t.Fatalf("Z21_ADDRESS = %q", envValue(env, "Z21_ADDRESS"))
	}
	if envValue(env, "NATS_URL") != "nats://nats.nats.svc:4222" {
		t.Fatalf("NATS_URL = %q", envValue(env, "NATS_URL"))
	}
	if envValue(env, "NATS_SUBJECT_PREFIX") != "z21.trains.basement" {
		t.Fatalf("NATS_SUBJECT_PREFIX = %q", envValue(env, "NATS_SUBJECT_PREFIX"))
	}
	if envValue(env, "Z21_DEVICE_NAMESPACE") != "trains" {
		t.Fatalf("Z21_DEVICE_NAMESPACE = %q", envValue(env, "Z21_DEVICE_NAMESPACE"))
	}
	if envValue(env, "Z21_BROADCAST_FLAGS") != "" {
		t.Fatalf("Z21_BROADCAST_FLAGS should not be set on the gateway deployment")
	}
}

func TestDesiredGatewayDeploymentSimulatorAddress(t *testing.T) {
	device := &z21v1alpha1.Z21Device{
		ObjectMeta: metav1.ObjectMeta{Name: "lab", Namespace: "default"},
		Spec: z21v1alpha1.Z21DeviceSpec{
			Backend: z21v1alpha1.BackendSpec{Type: z21v1alpha1.BackendSimulator},
			NATS:    z21v1alpha1.NatsSpec{URL: "nats://nats.default.svc:4222"},
		},
	}

	z21Addr, err := device.Z21Address()
	if err != nil {
		t.Fatal(err)
	}
	if z21Addr != "z21-sim-lab.elp.svc.cluster.local.:21105" {
		t.Fatalf("Z21Address = %q", z21Addr)
	}

	dep := desiredGatewayDeployment(device, "gateway:test", "z21-gateway-lab", z21Addr)
	if envValue(dep.Spec.Template.Spec.Containers[0].Env, "Z21_ADDRESS") != z21Addr {
		t.Fatal("gateway should dial simulator service DNS name")
	}
}

func TestDesiredSimulatorDeployment(t *testing.T) {
	device := &z21v1alpha1.Z21Device{
		ObjectMeta: metav1.ObjectMeta{Name: "lab", Namespace: "default"},
		Spec: z21v1alpha1.Z21DeviceSpec{
			Backend: z21v1alpha1.BackendSpec{
				Type: z21v1alpha1.BackendSimulator,
				Simulator: &z21v1alpha1.SimulatorBackend{
					Image: "z21-sim:local",
				},
			},
		},
	}

	dep := desiredSimulatorDeployment(device)
	if len(dep.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("containers = %d, want 1", len(dep.Spec.Template.Spec.Containers))
	}
	sim := dep.Spec.Template.Spec.Containers[0]
	if sim.Image != "z21-sim:local" {
		t.Fatalf("image = %q", sim.Image)
	}
	if sim.Ports[0].ContainerPort != z21v1alpha1.DefaultZ21Port {
		t.Fatalf("udp port = %d", sim.Ports[0].ContainerPort)
	}
	if sim.Ports[1].ContainerPort != z21v1alpha1.DefaultSimulatorGRPCPort {
		t.Fatalf("grpc port = %d", sim.Ports[1].ContainerPort)
	}
}

func TestSimulatorGRPCAddress(t *testing.T) {
	addr := z21v1alpha1.SimulatorGRPCAddress("elp", "lab")
	want := "z21-sim-lab.elp.svc.cluster.local.:50051"
	if addr != want {
		t.Fatalf("SimulatorGRPCAddress = %q, want %q", addr, want)
	}
}

func TestBroadcastFlagsDoNotChangeGatewayDeployment(t *testing.T) {
	base := &z21v1alpha1.Z21Device{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
		Spec: z21v1alpha1.Z21DeviceSpec{
			Backend: z21v1alpha1.BackendSpec{
				Type: z21v1alpha1.BackendHardware,
				Hardware: &z21v1alpha1.HardwareBackend{
					Host: "host.docker.internal",
				},
			},
			NATS:    z21v1alpha1.NatsSpec{URL: "nats://nats.default.svc.cluster.local:4222"},
			Gateway: z21v1alpha1.GatewaySpec{HostNetwork: true},
		},
	}

	flags := uint32(0x103)
	withFlags := base.DeepCopy()
	withFlags.Spec.BroadcastFlags = &flags

	z21Addr, err := base.Z21Address()
	if err != nil {
		t.Fatal(err)
	}

	without := desiredGatewayDeployment(base, "z21-gateway:local", "z21-gateway-main", z21Addr).Spec
	with := desiredGatewayDeployment(withFlags, "z21-gateway:local", "z21-gateway-main", z21Addr).Spec
	if !equality.Semantic.DeepEqual(without, with) {
		t.Fatal("broadcastFlags must not affect gateway deployment spec")
	}
}

func TestDeviceStatusIsCurrentRequiresGatewayReadyCondition(t *testing.T) {
	device := &z21v1alpha1.Z21Device{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
		Status: z21v1alpha1.Z21DeviceStatus{
			Phase:               z21v1alpha1.PhaseRunning,
			GatewayDeployment:   "z21-gateway-dev-01",
			SimulatorDeployment: "z21-sim-dev-01",
			SimulatorService:    "z21-sim-dev-01",
			ObservedGeneration:  1,
			Conditions: []metav1.Condition{
				{Type: z21v1alpha1.ConditionDeviceReachable, Status: metav1.ConditionTrue},
			},
		},
	}
	in := statusInput{
		gatewayDeploy:   &appsv1.Deployment{Status: appsv1.DeploymentStatus{ReadyReplicas: 1}},
		simulatorDeploy: &appsv1.Deployment{Status: appsv1.DeploymentStatus{ReadyReplicas: 1}},
		simulatorSvc:    "z21-sim-dev-01",
		phase:           z21v1alpha1.PhaseRunning,
	}
	if deviceStatusIsCurrent(device, in, "z21-gateway-dev-01", "z21-sim-dev-01", metav1.ConditionTrue) {
		t.Fatal("expected status patch when GatewayReady condition is missing")
	}

	setGatewayReadyCondition(device, metav1.ConditionTrue)
	if !deviceStatusIsCurrent(device, in, "z21-gateway-dev-01", "z21-sim-dev-01", metav1.ConditionTrue) {
		t.Fatal("expected no status patch when GatewayReady condition is present")
	}
}

func TestComputePhase(t *testing.T) {
	cases := []struct {
		name     string
		gateway  *appsv1DeploymentReady
		sim      *appsv1DeploymentReady
		stopping bool
		want     z21v1alpha1.DevicePhase
	}{
		{name: "stopping", stopping: true, want: z21v1alpha1.PhaseStopping},
		{name: "pending", want: z21v1alpha1.PhasePending},
		{name: "starting gateway", gateway: &appsv1DeploymentReady{ready: 0}, want: z21v1alpha1.PhaseStarting},
		{name: "running", gateway: &appsv1DeploymentReady{ready: 1}, want: z21v1alpha1.PhaseRunning},
		{name: "starting sim", gateway: &appsv1DeploymentReady{ready: 1}, sim: &appsv1DeploymentReady{ready: 0}, want: z21v1alpha1.PhaseStarting},
		{name: "running with sim", gateway: &appsv1DeploymentReady{ready: 1}, sim: &appsv1DeploymentReady{ready: 1}, want: z21v1alpha1.PhaseRunning},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computePhase(tc.gateway.deploy(), tc.sim.deploy(), tc.stopping)
			if got != tc.want {
				t.Fatalf("phase = %q, want %q", got, tc.want)
			}
		})
	}
}

type appsv1DeploymentReady struct {
	ready int32
}

func (d *appsv1DeploymentReady) deploy() *appsv1.Deployment {
	if d == nil {
		return nil
	}
	return &appsv1.Deployment{Status: appsv1.DeploymentStatus{ReadyReplicas: d.ready}}
}

func envValue(vars []corev1.EnvVar, name string) string {
	for _, v := range vars {
		if v.Name == name {
			return v.Value
		}
	}
	return ""
}
