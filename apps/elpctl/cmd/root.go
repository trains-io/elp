package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/trains-io/elp/apps/elpctl/internal/config"
)

var (
	serverURL     string
	namespace     string
	outputFmt     string
	elpConfigPath string
)

var rootCmd = &cobra.Command{
	Use:           "elpctl",
	Short:         "CLI for the elp control plane API",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return applyConfigDefaults(cmd)
	},
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		if printUsageError(err) {
			return err
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return err
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "", "elp API server URL (overrides elpconfig)")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace (overrides elpconfig)")
	rootCmd.PersistentFlags().StringVarP(&outputFmt, "output", "o", "table", "Output format: table or json")
	rootCmd.PersistentFlags().StringVar(&elpConfigPath, "elpconfig", "", "elpconfig file (default $ELPCONFIG or ~/.elp/config)")

	rootCmd.AddCommand(newDeviceCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newSimulateCmd())
	setUsageOnError(rootCmd)
}

func applyConfigDefaults(cmd *cobra.Command) error {
	if cmd.Name() == "help" || isConfigCommand(cmd) {
		return nil
	}

	if !cmd.Flags().Changed("server") {
		if v := os.Getenv("ELP_SERVER"); v != "" {
			serverURL = v
		} else {
			path, err := config.ResolvePath(elpConfigPath)
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			if cfg != nil {
				s, ns, err := cfg.Current()
				if err != nil {
					return err
				}
				serverURL = s
				if !cmd.Flags().Changed("namespace") && os.Getenv("ELP_NAMESPACE") == "" {
					namespace = ns
				}
			}
		}
	}
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	if !cmd.Flags().Changed("namespace") {
		if v := os.Getenv("ELP_NAMESPACE"); v != "" {
			namespace = v
		}
	}
	if namespace == "" {
		namespace = "default"
	}
	return nil
}

func isConfigCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "config" {
			return true
		}
	}
	return false
}
