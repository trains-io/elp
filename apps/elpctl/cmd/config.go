package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/trains-io/elp/apps/elpctl/internal/config"
	"gopkg.in/yaml.v3"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage elpconfig (kubeconfig-style API client configuration)",
	}
	cmd.AddCommand(
		newConfigViewCmd(),
		newConfigInitCmd(),
	)
	return cmd
}

func newConfigViewCmd() *cobra.Command {
	var rawPath string
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Display the current elpconfig",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ResolvePath(rawPath)
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			if cfg == nil {
				return fmt.Errorf("elpconfig not found at %s (run elpctl config init)", path)
			}
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return err
			}
			_, err = os.Stdout.Write(data)
			return err
		},
	}
	cmd.Flags().StringVar(&rawPath, "elpconfig", "", "elpconfig file (default $ELPCONFIG or ~/.elp/config)")
	return cmd
}

func newConfigInitCmd() *cobra.Command {
	var (
		rawPath          string
		kubeContext      string
		serviceName      string
		serviceNamespace string
		deviceNamespace  string
		dryRun           bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate elpconfig from the current Kubernetes cluster",
		Long: `Discover the elp-api Service external address via kubectl and write elpconfig.

Uses the active kubectl context for cluster/context names unless --context is set.
Requires kubectl access to the cluster where the API is deployed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			serverURL, ctxName, err := config.DiscoverFromCluster(config.DiscoverOptions{
				KubeContext:      kubeContext,
				ServiceName:      serviceName,
				ServiceNamespace: serviceNamespace,
			})
			if err != nil {
				return err
			}

			clusterName := ctxName
			contextName := fmt.Sprintf("%s@%s", ctxName, deviceNamespace)

			cfg := &config.Config{}
			if !dryRun {
				path, err := config.ResolvePath(rawPath)
				if err != nil {
					return err
				}
				existing, err := config.Load(path)
				if err != nil {
					return err
				}
				if existing != nil {
					cfg = existing
				}
			}
			cfg.UpsertClusterContext(clusterName, serverURL, contextName, deviceNamespace)

			if dryRun {
				data, err := yaml.Marshal(cfg)
				if err != nil {
					return err
				}
				_, err = os.Stdout.Write(data)
				return err
			}

			path, err := config.ResolvePath(rawPath)
			if err != nil {
				return err
			}
			if err := config.Write(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "elpconfig written to %s\n", path)
			fmt.Fprintf(os.Stderr, "current-context: %s\n", contextName)
			fmt.Fprintf(os.Stderr, "server: %s\n", serverURL)
			return nil
		},
	}

	cmd.Flags().StringVar(&rawPath, "elpconfig", "", "elpconfig file (default $ELPCONFIG or ~/.elp/config)")
	cmd.Flags().StringVar(&kubeContext, "context", "", "kubectl context name (default: current-context)")
	cmd.Flags().StringVar(&serviceName, "service", "elp-api", "API Service name")
	cmd.Flags().StringVar(&serviceNamespace, "service-namespace", "elp", "API Service namespace")
	cmd.Flags().StringVar(&deviceNamespace, "namespace", "default", "Target namespace for Z21Device resources")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print elpconfig to stdout instead of writing a file")
	return cmd
}
