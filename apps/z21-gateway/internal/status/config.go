package status

import (
	"os"

	"k8s.io/client-go/rest"
)

func loadInClusterConfig() (*rest.Config, error) {
	if host := os.Getenv("KUBERNETES_SERVICE_HOST"); host == "" {
		return nil, rest.ErrNotInCluster
	}
	return rest.InClusterConfig()
}
