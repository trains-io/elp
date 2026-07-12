package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	APIVersion = "v1"
	Kind       = "Config"
)

// Config mirrors kubeconfig: clusters, contexts, and a current context.
type Config struct {
	APIVersion      string    `yaml:"apiVersion"`
	Kind            string    `yaml:"kind"`
	Clusters        []Cluster `yaml:"clusters"`
	Contexts        []Context `yaml:"contexts"`
	CurrentContext  string    `yaml:"current-context"`
}

type Cluster struct {
	Name    string       `yaml:"name"`
	Cluster ClusterEntry `yaml:"cluster"`
}

type ClusterEntry struct {
	Server string `yaml:"server"`
}

type Context struct {
	Name    string       `yaml:"name"`
	Context ContextEntry `yaml:"context"`
}

type ContextEntry struct {
	Cluster   string `yaml:"cluster"`
	Namespace string `yaml:"namespace"`
}

// DefaultPath returns the elpconfig path from ELPCONFIG or ~/.elp/config.
func DefaultPath() (string, error) {
	if v := os.Getenv("ELPCONFIG"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".elp", "config"), nil
}

// ResolvePath picks an explicit flag value, ELPCONFIG, or the default path.
func ResolvePath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	return DefaultPath()
}

// Load reads an elpconfig file. A missing file returns (nil, nil).
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse elpconfig %s: %w", path, err)
	}
	return &cfg, nil
}

// Write persists cfg to path with kubeconfig-like permissions.
func Write(path string, cfg *Config) error {
	if cfg.APIVersion == "" {
		cfg.APIVersion = APIVersion
	}
	if cfg.Kind == "" {
		cfg.Kind = Kind
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Current returns the server URL and namespace for the current context.
func (c *Config) Current() (server, namespace string, err error) {
	if c == nil {
		return "", "", fmt.Errorf("no elpconfig loaded")
	}
	if c.CurrentContext == "" {
		return "", "", fmt.Errorf("current-context is not set")
	}
	ctx, ok := c.contextByName(c.CurrentContext)
	if !ok {
		return "", "", fmt.Errorf("context %q not found", c.CurrentContext)
	}
	cluster, ok := c.clusterByName(ctx.Context.Cluster)
	if !ok {
		return "", "", fmt.Errorf("cluster %q not found", ctx.Context.Cluster)
	}
	if cluster.Cluster.Server == "" {
		return "", "", fmt.Errorf("cluster %q has no server", ctx.Context.Cluster)
	}
	ns := ctx.Context.Namespace
	if ns == "" {
		ns = "default"
	}
	return cluster.Cluster.Server, ns, nil
}

// UpsertClusterContext adds or updates cluster/context entries and sets current-context.
func (c *Config) UpsertClusterContext(clusterName, server, contextName, namespace string) {
	if c.APIVersion == "" {
		c.APIVersion = APIVersion
	}
	if c.Kind == "" {
		c.Kind = Kind
	}
	if namespace == "" {
		namespace = "default"
	}

	found := false
	for i := range c.Clusters {
		if c.Clusters[i].Name == clusterName {
			c.Clusters[i].Cluster.Server = server
			found = true
			break
		}
	}
	if !found {
		c.Clusters = append(c.Clusters, Cluster{
			Name: clusterName,
			Cluster: ClusterEntry{
				Server: server,
			},
		})
	}

	found = false
	for i := range c.Contexts {
		if c.Contexts[i].Name == contextName {
			c.Contexts[i].Context.Cluster = clusterName
			c.Contexts[i].Context.Namespace = namespace
			found = true
			break
		}
	}
	if !found {
		c.Contexts = append(c.Contexts, Context{
			Name: contextName,
			Context: ContextEntry{
				Cluster:   clusterName,
				Namespace: namespace,
			},
		})
	}
	c.CurrentContext = contextName
}

func (c *Config) contextByName(name string) (Context, bool) {
	for _, ctx := range c.Contexts {
		if ctx.Name == name {
			return ctx, true
		}
	}
	return Context{}, false
}

func (c *Config) clusterByName(name string) (Cluster, bool) {
	for _, cluster := range c.Clusters {
		if cluster.Name == name {
			return cluster, true
		}
	}
	return Cluster{}, false
}
