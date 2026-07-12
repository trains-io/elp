package controller

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

const (
	defaultGatewayImage      = "ghcr.io/trains-io/z21-gateway:latest"
	defaultWorkloadNamespace = "elp"
	gatewayComponent         = "z21-gateway"
	simulatorComponent       = "z21-sim"
)

// Z21DeviceReconciler reconciles a Z21Device object by managing its gateway workload.
type Z21DeviceReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	GatewayImage      string
	WorkloadNamespace string
}

// +kubebuilder:rbac:groups=z21.trains.io,resources=z21devices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=z21.trains.io,resources=z21devices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=z21.trains.io,resources=z21devices/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *Z21DeviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var device z21v1alpha1.Z21Device
	if err := r.Get(ctx, req.NamespacedName, &device); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(&device, z21v1alpha1.DeviceFinalizerName) {
		controllerutil.AddFinalizer(&device, z21v1alpha1.DeviceFinalizerName)
		if err := r.Update(ctx, &device); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !device.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &device)
	}

	saName, err := r.reconcileGatewayServiceAccount(ctx, &device)
	if err != nil {
		logger.Error(err, "failed to reconcile gateway service account")
		return r.updateStatus(ctx, &device, statusInput{phase: z21v1alpha1.PhaseFailed})
	}

	if err := r.reconcileGatewayRBAC(ctx, &device, saName); err != nil {
		logger.Error(err, "failed to reconcile gateway RBAC")
		return r.updateStatus(ctx, &device, statusInput{phase: z21v1alpha1.PhaseFailed})
	}

	simDeploy, simService, err := r.reconcileSimulator(ctx, &device)
	if err != nil {
		logger.Error(err, "failed to reconcile simulator")
		return r.updateStatus(ctx, &device, statusInput{
			simulatorDeploy: simDeploy,
			simulatorSvc:    simService,
			phase:           z21v1alpha1.PhaseFailed,
		})
	}

	deploy, err := r.reconcileGatewayDeployment(ctx, &device, saName)
	if err != nil {
		logger.Error(err, "failed to reconcile gateway deployment")
		return r.updateStatus(ctx, &device, statusInput{
			gatewayDeploy:   deploy,
			simulatorDeploy: simDeploy,
			simulatorSvc:    simService,
			phase:           z21v1alpha1.PhaseFailed,
		})
	}

	phase := computePhase(deploy, simDeploy, false)
	return r.updateStatus(ctx, &device, statusInput{
		gatewayDeploy:   deploy,
		simulatorDeploy: simDeploy,
		simulatorSvc:    simService,
		phase:           phase,
	})
}

func (r *Z21DeviceReconciler) reconcileDelete(ctx context.Context, device *z21v1alpha1.Z21Device) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if _, err := r.updateStatus(ctx, device, statusInput{phase: z21v1alpha1.PhaseStopping}); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.deleteGateway(ctx, device); err != nil {
		logger.Error(err, "failed to delete gateway resources")
		return ctrl.Result{}, err
	}

	if err := r.deleteSimulator(ctx, device); err != nil {
		logger.Error(err, "failed to delete simulator resources")
		return ctrl.Result{}, err
	}

	latest := &z21v1alpha1.Z21Device{}
	if err := r.Get(ctx, types.NamespacedName{Name: device.Name, Namespace: device.Namespace}, latest); err != nil {
		return ctrl.Result{}, err
	}
	controllerutil.RemoveFinalizer(latest, z21v1alpha1.DeviceFinalizerName)
	if err := r.Update(ctx, latest); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *Z21DeviceReconciler) reconcileSimulator(
	ctx context.Context,
	device *z21v1alpha1.Z21Device,
) (*appsv1.Deployment, string, error) {
	if device.Spec.Backend.Type != z21v1alpha1.BackendSimulator {
		if err := r.deleteSimulator(ctx, device); err != nil {
			return nil, "", err
		}
		return nil, "", nil
	}

	svcName, err := r.reconcileSimulatorService(ctx, device)
	if err != nil {
		return nil, "", err
	}

	deploy, err := r.reconcileSimulatorDeployment(ctx, device)
	if err != nil {
		return deploy, svcName, err
	}
	return deploy, svcName, nil
}

func (r *Z21DeviceReconciler) reconcileSimulatorService(ctx context.Context, device *z21v1alpha1.Z21Device) (string, error) {
	name := z21v1alpha1.SimulatorServiceName(device.Name)
	labels := simulatorLabels(device)
	selector := simulatorSelector(device)

	svc := &corev1.Service{}
	svc.Name = name
	svc.Namespace = device.Namespace

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		if err := controllerutil.SetControllerReference(device, svc, r.Scheme); err != nil {
			return err
		}
		svc.Labels = labels
		svc.Spec = corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: selector,
			Ports: []corev1.ServicePort{{
				Name:       "z21-udp",
				Port:       z21v1alpha1.DefaultZ21Port,
				TargetPort: intstr.FromInt32(z21v1alpha1.DefaultZ21Port),
				Protocol:   corev1.ProtocolUDP,
			}},
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("create or update simulator service: %w", err)
	}
	return name, nil
}

func (r *Z21DeviceReconciler) reconcileSimulatorDeployment(ctx context.Context, device *z21v1alpha1.Z21Device) (*appsv1.Deployment, error) {
	name := simulatorDeploymentName(device)
	labels := simulatorLabels(device)

	deploy := &appsv1.Deployment{}
	deploy.Name = name
	deploy.Namespace = device.Namespace

	desired := desiredSimulatorDeployment(device)
	desired.Name = name
	desired.Namespace = device.Namespace
	desired.Labels = labels

	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: device.Namespace}, deploy); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("get simulator deployment: %w", err)
		}
		deploy = desired
		deploy.Labels = labels
		if err := controllerutil.SetControllerReference(device, deploy, r.Scheme); err != nil {
			return nil, err
		}
		if err := r.Create(ctx, deploy); err != nil {
			return nil, fmt.Errorf("create simulator deployment: %w", err)
		}
		log.FromContext(ctx).Info("reconciled simulator deployment", "operation", "created", "deployment", name)
		return deploy, nil
	}

	if equality.Semantic.DeepEqual(deploy.Spec, desired.Spec) {
		return deploy, nil
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		if err := controllerutil.SetControllerReference(device, deploy, r.Scheme); err != nil {
			return err
		}
		deploy.Labels = labels
		deploy.Spec = desired.Spec
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create or update simulator deployment: %w", err)
	}

	log.FromContext(ctx).Info("reconciled simulator deployment", "operation", op, "deployment", name)
	return deploy, nil
}

func (r *Z21DeviceReconciler) deleteSimulator(ctx context.Context, device *z21v1alpha1.Z21Device) error {
	deployName := simulatorDeploymentName(device)
	svcName := z21v1alpha1.SimulatorServiceName(device.Name)

	deploy := &appsv1.Deployment{}
	deploy.Name = deployName
	deploy.Namespace = device.Namespace
	if err := r.Delete(ctx, deploy); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete simulator deployment: %w", err)
	}

	svc := &corev1.Service{}
	svc.Name = svcName
	svc.Namespace = device.Namespace
	if err := r.Delete(ctx, svc); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete simulator service: %w", err)
	}
	return nil
}

func (r *Z21DeviceReconciler) reconcileGatewayServiceAccount(ctx context.Context, device *z21v1alpha1.Z21Device) (string, error) {
	name := gatewayServiceAccountName(device)
	labels := gatewayLabels(device)
	ns := r.gatewayNamespace()

	sa := &corev1.ServiceAccount{}
	sa.Name = name
	sa.Namespace = ns

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error {
		sa.Labels = labels
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("create or update service account: %w", err)
	}
	return name, nil
}

func (r *Z21DeviceReconciler) reconcileGatewayRBAC(ctx context.Context, device *z21v1alpha1.Z21Device, saName string) error {
	labels := gatewayLabels(device)
	roleName := gatewayRoleName(device)
	bindingName := gatewayRoleBindingName(device)

	role := &rbacv1.Role{}
	role.Name = roleName
	role.Namespace = device.Namespace
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, role, func() error {
		if err := controllerutil.SetControllerReference(device, role, r.Scheme); err != nil {
			return err
		}
		role.Labels = labels
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{"z21.trains.io"},
				Resources: []string{"z21devices"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"z21.trains.io"},
				Resources: []string{"z21devices/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
		}
		return nil
	}); err != nil {
		return fmt.Errorf("create or update role: %w", err)
	}

	binding := &rbacv1.RoleBinding{}
	binding.Name = bindingName
	binding.Namespace = device.Namespace
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, binding, func() error {
		if err := controllerutil.SetControllerReference(device, binding, r.Scheme); err != nil {
			return err
		}
		binding.Labels = labels
		binding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     roleName,
		}
		binding.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      saName,
			Namespace: r.gatewayNamespace(),
		}}
		return nil
	}); err != nil {
		return fmt.Errorf("create or update role binding: %w", err)
	}

	return nil
}

func (r *Z21DeviceReconciler) reconcileGatewayDeployment(ctx context.Context, device *z21v1alpha1.Z21Device, saName string) (*appsv1.Deployment, error) {
	name := gatewayDeploymentName(device)
	labels := gatewayLabels(device)

	z21Address, err := device.Z21Address()
	if err != nil {
		return nil, err
	}

	deploy := &appsv1.Deployment{}
	deploy.Name = name
	deploy.Namespace = r.gatewayNamespace()

	desired := desiredGatewayDeployment(device, r.gatewayImage(), saName, z21Address)
	desired.Name = name
	desired.Namespace = r.gatewayNamespace()
	desired.Labels = labels

	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: r.gatewayNamespace()}, deploy); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("get deployment: %w", err)
		}
		deploy = desired
		deploy.Labels = labels
		if err := r.Create(ctx, deploy); err != nil {
			return nil, fmt.Errorf("create deployment: %w", err)
		}
		log.FromContext(ctx).Info("reconciled gateway deployment", "operation", "created", "deployment", name)
		return deploy, nil
	}

	if equality.Semantic.DeepEqual(deploy.Spec, desired.Spec) {
		return deploy, nil
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		deploy.Labels = labels
		deploy.Spec = desired.Spec
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create or update deployment: %w", err)
	}

	log.FromContext(ctx).Info("reconciled gateway deployment", "operation", op, "deployment", name)
	return deploy, nil
}

type statusInput struct {
	gatewayDeploy   *appsv1.Deployment
	simulatorDeploy *appsv1.Deployment
	simulatorSvc    string
	phase           z21v1alpha1.DevicePhase
}

func (r *Z21DeviceReconciler) updateStatus(
	ctx context.Context,
	device *z21v1alpha1.Z21Device,
	in statusInput,
) (ctrl.Result, error) {
	latest := &z21v1alpha1.Z21Device{}
	if err := r.Get(ctx, types.NamespacedName{Name: device.Name, Namespace: device.Namespace}, latest); err != nil {
		return ctrl.Result{}, err
	}

	gatewayName := ""
	if in.gatewayDeploy != nil {
		gatewayName = in.gatewayDeploy.Name
	}
	simulatorName := ""
	if in.simulatorDeploy != nil {
		simulatorName = in.simulatorDeploy.Name
	}

	gatewayReady := gatewayReadyStatus(in.gatewayDeploy)
	setGatewayReadyCondition(latest, gatewayReady)

	if latest.Status.Phase == in.phase &&
		latest.Status.GatewayDeployment == gatewayName &&
		latest.Status.SimulatorDeployment == simulatorName &&
		latest.Status.SimulatorService == in.simulatorSvc &&
		latest.Status.ObservedGeneration == latest.Generation &&
		conditionStatus(latest.Status.Conditions, z21v1alpha1.ConditionGatewayReady) == gatewayReady {
		return ctrl.Result{}, nil
	}

	patchBase := latest.DeepCopy()
	latest.Status.Phase = in.phase
	latest.Status.GatewayDeployment = gatewayName
	latest.Status.SimulatorDeployment = simulatorName
	latest.Status.SimulatorService = in.simulatorSvc
	latest.Status.ObservedGeneration = latest.Generation
	if err := r.Status().Patch(ctx, latest, client.MergeFrom(patchBase)); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func setGatewayReadyCondition(device *z21v1alpha1.Z21Device, status metav1.ConditionStatus) {
	reason := "GatewayNotReady"
	message := "gateway deployment has no ready replicas"
	if status == metav1.ConditionTrue {
		reason = "GatewayReady"
		message = "gateway deployment has at least one ready replica"
	}
	meta.SetStatusCondition(&device.Status.Conditions, metav1.Condition{
		Type:               z21v1alpha1.ConditionGatewayReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: device.Generation,
	})
}

func gatewayReadyStatus(deploy *appsv1.Deployment) metav1.ConditionStatus {
	if deploy != nil && deploy.Status.ReadyReplicas >= 1 {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func conditionStatus(conditions []metav1.Condition, condType string) metav1.ConditionStatus {
	for _, cond := range conditions {
		if cond.Type == condType {
			return cond.Status
		}
	}
	return ""
}

func computePhase(gatewayDeploy, simDeploy *appsv1.Deployment, stopping bool) z21v1alpha1.DevicePhase {
	if stopping {
		return z21v1alpha1.PhaseStopping
	}
	if deploymentFailed(gatewayDeploy) || deploymentFailed(simDeploy) {
		return z21v1alpha1.PhaseFailed
	}

	gatewayReady := gatewayDeploy != nil && gatewayDeploy.Status.ReadyReplicas >= 1
	simReady := simDeploy == nil || simDeploy.Status.ReadyReplicas >= 1
	if gatewayReady && simReady {
		return z21v1alpha1.PhaseRunning
	}

	if gatewayDeploy != nil || simDeploy != nil {
		return z21v1alpha1.PhaseStarting
	}
	return z21v1alpha1.PhasePending
}

func deploymentFailed(deploy *appsv1.Deployment) bool {
	if deploy == nil {
		return false
	}
	for _, cond := range deploy.Status.Conditions {
		if cond.Type == appsv1.DeploymentProgressing &&
			cond.Status == corev1.ConditionFalse &&
			cond.Reason == "ProgressDeadlineExceeded" {
			return true
		}
	}
	return false
}

func (r *Z21DeviceReconciler) deleteGateway(ctx context.Context, device *z21v1alpha1.Z21Device) error {
	ns := r.gatewayNamespace()
	deployName := gatewayDeploymentName(device)
	saName := gatewayServiceAccountName(device)

	deploy := &appsv1.Deployment{}
	deploy.Name = deployName
	deploy.Namespace = ns
	if err := r.Delete(ctx, deploy); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete gateway deployment: %w", err)
	}

	sa := &corev1.ServiceAccount{}
	sa.Name = saName
	sa.Namespace = ns
	if err := r.Delete(ctx, sa); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete gateway service account: %w", err)
	}
	return nil
}

func (r *Z21DeviceReconciler) gatewayNamespace() string {
	if r.WorkloadNamespace != "" {
		return r.WorkloadNamespace
	}
	return defaultWorkloadNamespace
}

func (r *Z21DeviceReconciler) gatewayImage() string {
	if r.GatewayImage != "" {
		return r.GatewayImage
	}
	return defaultGatewayImage
}

func (r *Z21DeviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&z21v1alpha1.Z21Device{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}

func gatewayDeploymentName(device *z21v1alpha1.Z21Device) string {
	return fmt.Sprintf("z21-gateway-%s", device.Name)
}

func simulatorDeploymentName(device *z21v1alpha1.Z21Device) string {
	return fmt.Sprintf("z21-sim-%s", device.Name)
}

func gatewayServiceAccountName(device *z21v1alpha1.Z21Device) string {
	return fmt.Sprintf("z21-gateway-%s", device.Name)
}

func gatewayRoleName(device *z21v1alpha1.Z21Device) string {
	return fmt.Sprintf("z21-gateway-%s", device.Name)
}

func gatewayRoleBindingName(device *z21v1alpha1.Z21Device) string {
	return fmt.Sprintf("z21-gateway-%s", device.Name)
}

func gatewayLabels(device *z21v1alpha1.Z21Device) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       gatewayComponent,
		"app.kubernetes.io/instance":   device.Name,
		"app.kubernetes.io/managed-by": "z21-device-controller",
		"z21.trains.io/device":         device.Name,
	}
}

func simulatorLabels(device *z21v1alpha1.Z21Device) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       simulatorComponent,
		"app.kubernetes.io/instance":   device.Name,
		"app.kubernetes.io/managed-by": "z21-device-controller",
		"z21.trains.io/device":         device.Name,
	}
}

func simulatorSelector(device *z21v1alpha1.Z21Device) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     simulatorComponent,
		"app.kubernetes.io/instance": device.Name,
	}
}

func desiredSimulatorDeployment(device *z21v1alpha1.Z21Device) *appsv1.Deployment {
	labels := simulatorLabels(device)
	selector := simulatorSelector(device)
	replicas := int32(1)

	return &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            simulatorComponent,
						Image:           device.SimulatorImage(),
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{{
							Name:          "z21-udp",
							ContainerPort: z21v1alpha1.DefaultZ21Port,
							Protocol:      corev1.ProtocolUDP,
						}},
					}},
				},
			},
		},
	}
}

func desiredGatewayDeployment(device *z21v1alpha1.Z21Device, image, saName, z21Address string) *appsv1.Deployment {
	if device.Spec.Gateway.Image != "" {
		image = device.Spec.Gateway.Image
	}

	labels := gatewayLabels(device)
	selector := map[string]string{
		"app.kubernetes.io/name":     gatewayComponent,
		"app.kubernetes.io/instance": device.Name,
	}

	replicas := int32(1)
	podSpec := corev1.PodSpec{
		ServiceAccountName: saName,
		HostNetwork:        device.Spec.Gateway.HostNetwork,
		NodeSelector:       device.Spec.Gateway.NodeSelector,
		Containers: []corev1.Container{
			{
				Name:            gatewayComponent,
				Image:           image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Env: []corev1.EnvVar{
					{Name: "Z21_ADDRESS", Value: z21Address},
					{Name: "NATS_URL", Value: device.Spec.NATS.URL},
					{Name: "NATS_SUBJECT_PREFIX", Value: device.SubjectPrefix()},
					{Name: "Z21_DEVICE_NAME", Value: device.Name},
					{Name: "Z21_DEVICE_NAMESPACE", Value: device.Namespace},
				},
				Ports: []corev1.ContainerPort{
					{Name: "health", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromString("health"),
						},
					},
					InitialDelaySeconds: 5,
					PeriodSeconds:       10,
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/readyz",
							Port: intstr.FromString("health"),
						},
					},
					InitialDelaySeconds: 2,
					PeriodSeconds:       5,
				},
			},
		},
	}
	if device.Spec.Gateway.HostNetwork {
		// hostNetwork pods otherwise use the node's resolver (Docker/WSL) and cannot
		// resolve in-cluster service names such as nats.default.svc.cluster.local.
		podSpec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	}
	podSpec.HostAliases = append([]corev1.HostAlias(nil), device.Spec.Gateway.HostAliases...)
	if device.Spec.Backend.Type == z21v1alpha1.BackendHardware &&
		device.Spec.Backend.Hardware != nil &&
		strings.EqualFold(device.Spec.Backend.Hardware.Host, "host.docker.internal") &&
		!hasHostAlias(podSpec.HostAliases, "host.docker.internal") {
		// Kind/WSL dev default; override with gateway.hostAliases for other environments.
		podSpec.HostAliases = append(podSpec.HostAliases, corev1.HostAlias{
			IP:        "172.18.0.1",
			Hostnames: []string{"host.docker.internal"},
		})
	}

	return &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       podSpec,
			},
		},
	}
}

func hasHostAlias(aliases []corev1.HostAlias, host string) bool {
	for _, alias := range aliases {
		for _, name := range alias.Hostnames {
			if strings.EqualFold(name, host) {
				return true
			}
		}
	}
	return false
}
