package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWriteCurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")

	cfg := &Config{}
	cfg.UpsertClusterContext("kind-elp", "http://172.18.255.200:8080", "kind-elp@default", "default")
	if err := Write(path, cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	server, ns, err := loaded.Current()
	if err != nil {
		t.Fatal(err)
	}
	if server != "http://172.18.255.200:8080" || ns != "default" {
		t.Fatalf("current = %q %q", server, ns)
	}
	if loaded.CurrentContext != "kind-elp@default" {
		t.Fatalf("current-context = %q", loaded.CurrentContext)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestUpsertUpdatesExisting(t *testing.T) {
	cfg := &Config{}
	cfg.UpsertClusterContext("kind-elp", "http://old:8080", "kind-elp@default", "default")
	cfg.UpsertClusterContext("kind-elp", "http://new:8080", "kind-elp@default", "staging")

	server, ns, err := cfg.Current()
	if err != nil {
		t.Fatal(err)
	}
	if server != "http://new:8080" || ns != "staging" {
		t.Fatalf("current = %q %q", server, ns)
	}
	if len(cfg.Clusters) != 1 || len(cfg.Contexts) != 1 {
		t.Fatalf("clusters/contexts = %d %d", len(cfg.Clusters), len(cfg.Contexts))
	}
}

func TestDefaultPathEnv(t *testing.T) {
	t.Setenv("ELPCONFIG", "/tmp/custom-elp-config")
	path, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if path != "/tmp/custom-elp-config" {
		t.Fatalf("path = %q", path)
	}
}

func TestUpsertEmptyNamespaceUsesDefault(t *testing.T) {
	cfg := &Config{}
	cfg.UpsertClusterContext("kind-elp", "http://localhost:8080", "kind-elp@elp", "")

	_, ns, err := cfg.Current()
	if err != nil {
		t.Fatal(err)
	}
	if ns != DefaultDeviceNamespace {
		t.Fatalf("namespace = %q, want %q", ns, DefaultDeviceNamespace)
	}
}

func TestWritePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config")
	cfg := &Config{}
	cfg.UpsertClusterContext("elp", "http://localhost:8080", "elp@default", "default")
	if err := Write(path, cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}
