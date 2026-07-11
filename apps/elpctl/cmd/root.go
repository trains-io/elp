package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	serverURL string
	namespace string
	outputFmt string
)

var rootCmd = &cobra.Command{
	Use:           "elpctl",
	Short:         "CLI for the elp control plane API",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return err
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", envOr("ELP_SERVER", "http://localhost:8080"), "elp API server URL")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", envOr("ELP_NAMESPACE", "default"), "Kubernetes namespace")
	rootCmd.PersistentFlags().StringVarP(&outputFmt, "output", "o", "table", "Output format: table or json")

	rootCmd.AddCommand(newDeviceCmd())
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
