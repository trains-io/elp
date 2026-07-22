package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/trains-io/elp/apps/elpctl/internal/client"
	"github.com/trains-io/elp/apps/elpctl/internal/simulation"
)

func newSimulateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "simulate",
		Short: "Simulate Z21 device behavior on simulator backends",
	}
	cmd.AddCommand(newSimulateCANCmd())
	return cmd
}

func newSimulateCANCmd() *cobra.Command {
	var (
		simulationName string
		netIDs         []string
		detectorName   string
		moduleAddress  uint16
		portCount      uint16
	)

	cmd := &cobra.Command{
		Use:   "can DEVICE",
		Short: "Simulate CAN device detection on a simulator-backed Z21 device",
		Long: `Register simulated CAN modules on a z21-sim backend by creating or updating a Simulation custom resource.

The elp API applies the Simulation CR to the cluster; z21-sim-controller then drives the simulator over gRPC.`,
		Args: ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deviceName := args[0]

			parsedNetIDs := make([]uint16, 0, len(netIDs))
			for _, raw := range netIDs {
				netID, err := simulation.ParseNetID(raw)
				if err != nil {
					return usageErr(cmd, err.Error())
				}
				parsedNetIDs = append(parsedNetIDs, netID)
			}

			detectors := simulation.DetectorsFromNetIDs(parsedNetIDs)
			if len(detectors) == 1 && detectorName != "" {
				detectors[0].Name = detectorName
			}
			if len(detectors) == 1 && cmd.Flags().Changed("module-address") {
				detectors[0].ModuleAddress = moduleAddress
			}
			if len(detectors) == 1 && cmd.Flags().Changed("port-count") {
				detectors[0].PortCount = portCount
			}

			apiClient := newAPIClient()
			sim, err := apiClient.SimulateCAN(cmd.Context(), deviceName, client.SimulateCANRequest{
				SimulationName: simulationName,
				CANDetectors:   detectors,
			})
			if err != nil {
				return err
			}
			return printSimulation(sim, outputFmt)
		},
	}

	cmd.Flags().StringVar(&simulationName, "simulation-name", "", "Simulation resource name (default: device name)")
	cmd.Flags().StringArrayVar(&netIDs, "net-id", nil, "CAN detector net ID, decimal or 0x hex (repeatable; default 0xDB04)")
	cmd.Flags().StringVar(&detectorName, "detector-name", "", "Detector label when a single --net-id is used")
	cmd.Flags().Uint16Var(&moduleAddress, "module-address", 1, "Module address when a single --net-id is used")
	cmd.Flags().Uint16Var(&portCount, "port-count", 8, "Feedback port count when a single --net-id is used")

	return cmd
}

func printSimulation(sim client.Simulation, format string) error {
	if format == "json" {
		return json.NewEncoder(os.Stdout).Encode(sim)
	}
	phase := "-"
	applied := "-"
	if sim.Status != nil {
		if sim.Status.Phase != "" {
			phase = sim.Status.Phase
		}
		applied = fmt.Sprintf("%d", sim.Status.AppliedCANDevices)
	}
	_, err := fmt.Fprintf(os.Stdout, "simulation %q applied for device %q (%d can detector(s), phase=%s, applied=%s)\n",
		sim.Name, sim.DeviceRef, len(sim.CANDetectors), phase, applied)
	return err
}
