package config

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// DiscoverOptions tune how the API server URL is resolved from Kubernetes.
type DiscoverOptions struct {
	KubeContext       string
	ServiceName       string
	ServiceNamespace  string
	KubectlBinary     string
}

// DiscoverFromCluster reads the elp-api Service using kubectl and the active kube context.
func DiscoverFromCluster(opts DiscoverOptions) (serverURL, kubeContext string, err error) {
	if opts.ServiceName == "" {
		opts.ServiceName = "elp-api"
	}
	if opts.ServiceNamespace == "" {
		opts.ServiceNamespace = "elp"
	}
	if opts.KubectlBinary == "" {
		opts.KubectlBinary = "kubectl"
	}

	kubeContext = opts.KubeContext
	if kubeContext == "" {
		kubeContext, err = kubectlOutput(opts.KubectlBinary, "config", "current-context")
		if err != nil {
			return "", "", fmt.Errorf("kubectl current-context: %w", err)
		}
		kubeContext = strings.TrimSpace(kubeContext)
		if kubeContext == "" {
			return "", "", fmt.Errorf("kubectl has no current context")
		}
	}

	raw, err := kubectlOutput(opts.KubectlBinary, "get", "svc", opts.ServiceName,
		"-n", opts.ServiceNamespace,
		"-o", "json")
	if err != nil {
		return "", "", fmt.Errorf("get service/%s: %w", opts.ServiceName, err)
	}

	var svc serviceJSON
	if err := json.Unmarshal([]byte(raw), &svc); err != nil {
		return "", "", fmt.Errorf("decode service: %w", err)
	}

	port := 8080
	if len(svc.Spec.Ports) > 0 && svc.Spec.Ports[0].Port > 0 {
		port = svc.Spec.Ports[0].Port
	}

	host, err := serviceExternalHost(svc)
	if err != nil {
		return "", "", err
	}

	serverURL = fmt.Sprintf("http://%s:%d", host, port)
	return serverURL, kubeContext, nil
}

type serviceJSON struct {
	Spec struct {
		Ports []struct {
			Port int `json:"port"`
		} `json:"ports"`
	} `json:"spec"`
	Status struct {
		LoadBalancer struct {
			Ingress []struct {
				IP       string `json:"ip"`
				Hostname string `json:"hostname"`
			} `json:"ingress"`
		} `json:"loadBalancer"`
	} `json:"status"`
}

func serviceExternalHost(svc serviceJSON) (string, error) {
	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		if ingress.IP != "" {
			return ingress.IP, nil
		}
		if ingress.Hostname != "" {
			return ingress.Hostname, nil
		}
	}
	return "", fmt.Errorf("service has no LoadBalancer external address yet; wait for MetalLB or use port-forward")
}

func kubectlOutput(binary string, args ...string) (string, error) {
	cmd := exec.Command(binary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, msg)
	}
	return string(out), nil
}
