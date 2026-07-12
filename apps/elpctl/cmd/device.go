package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trains-io/elp/apps/elpctl/internal/client"
)

func newDeviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "device",
		Aliases: []string{"devices"},
		Short:   "Manage Z21 devices",
	}
	cmd.AddCommand(
		newDeviceCreateCmd(),
		newDeviceGetCmd(),
		newDeviceListCmd(),
		newDeviceWatchCmd(),
	)
	return cmd
}

func newDeviceCreateCmd() *cobra.Command {
	var (
		backendType    string
		address        string
		natsURL        string
		subjectPrefix  string
		hostNetwork    bool
		gatewayImage   string
		simulatorImage string
		nodeSelector   []string
		broadcastFlags uint32
		setBroadcast   bool
	)

	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Register a new Z21 device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := client.DeviceCreate{
				Name: args[0],
				NATS: client.NatsConfig{
					URL:           natsURL,
					SubjectPrefix: subjectPrefix,
				},
			}

			switch backendType {
			case "hardware":
				if address == "" {
					return fmt.Errorf("--address is required for hardware backend")
				}
				req.Address = address
			case "simulator":
				sim := &client.SimulatorCreate{}
				if simulatorImage != "" {
					sim.Image = simulatorImage
				}
				req.Backend = &client.BackendCreate{
					Type:      "simulator",
					Simulator: sim,
				}
			default:
				return fmt.Errorf("unsupported backend type %q", backendType)
			}

			if hostNetwork || gatewayImage != "" || len(nodeSelector) > 0 {
				req.Gateway = &client.GatewayConfig{
					Image:        gatewayImage,
					HostNetwork:  hostNetwork,
					NodeSelector: parseNodeSelector(nodeSelector),
				}
			}
			if setBroadcast {
				flags := broadcastFlags
				req.BroadcastFlags = &flags
			}

			c := newAPIClient()
			device, err := c.CreateDevice(cmd.Context(), req)
			if err != nil {
				return err
			}
			return printDevice(device, outputFmt)
		},
	}

	cmd.Flags().StringVar(&backendType, "backend", "hardware", "Backend type: hardware or simulator")
	cmd.Flags().StringVar(&address, "address", "", "Z21 UDP endpoint host:port (required for hardware backend)")
	cmd.Flags().StringVar(&simulatorImage, "simulator-image", "", "Simulator container image (simulator backend only)")
	cmd.Flags().StringVar(&natsURL, "nats-url", "nats://nats.default.svc.cluster.local.:4222", "NATS URL for gateway events and control")
	cmd.Flags().StringVar(&subjectPrefix, "nats-subject-prefix", "", "NATS subject prefix override")
	cmd.Flags().BoolVar(&hostNetwork, "host-network", false, "Run gateway with hostNetwork")
	cmd.Flags().StringVar(&gatewayImage, "gateway-image", "", "Gateway container image")
	cmd.Flags().StringArrayVar(&nodeSelector, "node-selector", nil, "Gateway nodeSelector as key=value (repeatable)")
	cmd.Flags().Uint32Var(&broadcastFlags, "broadcast-flags", 0, "LAN_SET_BROADCASTFLAGS bitmask")
	cmd.Flags().BoolVar(&setBroadcast, "set-broadcast-flags", false, "Apply --broadcast-flags (otherwise API default applies)")

	return cmd
}

func newDeviceGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get NAME",
		Short: "Show a Z21 device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newAPIClient()
			device, err := c.GetDevice(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printDevice(device, outputFmt)
		},
	}
	return cmd
}

func newDeviceListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List Z21 devices",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newAPIClient()
			list, err := c.ListDevices(cmd.Context())
			if err != nil {
				return err
			}
			return printDeviceList(list.Items, outputFmt)
		},
	}
	return cmd
}

func newDeviceWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch [NAME]",
		Short: "Watch Z21 device status changes (SSE)",
		Long:  "Streams device updates from the API. With NAME, only events for that device are shown.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var filter string
			if len(args) == 1 {
				filter = args[0]
			}

			c := newAPIClient()
			ctx := cmd.Context()
			watchOut := newDeviceWatchWriter(os.Stdout)
			return c.WatchDevices(ctx, func(event client.StreamEvent) error {
				switch event.Type {
				case "snapshot":
					items := event.Items
					if filter != "" {
						items = filterDevices(items, filter)
					}
					if outputFmt == "json" {
						return json.NewEncoder(os.Stdout).Encode(client.DeviceList{Items: items})
					}
					return watchOut.writeSnapshot(items)
				case "updated":
					if filter != "" && event.Device.Name != filter {
						return nil
					}
					if outputFmt == "json" {
						return json.NewEncoder(os.Stdout).Encode(event.Device)
					}
					return watchOut.writeUpdated(event.Device)
				case "deleted":
					if filter != "" && event.Name != filter {
						return nil
					}
					if outputFmt == "json" {
						return json.NewEncoder(os.Stdout).Encode(map[string]string{
							"type": "deleted",
							"name": event.Name,
						})
					}
					return watchOut.writeDeleted(event.Name)
				default:
					if outputFmt == "json" {
						return json.NewEncoder(os.Stdout).Encode(event)
					}
					return nil
				}
			})
		},
	}
	return cmd
}

func newAPIClient() *client.Client {
	return client.New(serverURL, namespace)
}

func parseNodeSelector(pairs []string) map[string]string {
	if len(pairs) == 0 {
		return nil
	}
	out := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func filterDevices(items []client.Device, name string) []client.Device {
	out := make([]client.Device, 0, 1)
	for _, item := range items {
		if item.Name == name {
			out = append(out, item)
		}
	}
	return out
}

func printDeviceList(items []client.Device, format string) error {
	if format == "json" {
		return json.NewEncoder(os.Stdout).Encode(client.DeviceList{Items: items})
	}
	return printDeviceTable(items)
}

func printDevice(device client.Device, format string) error {
	if format == "json" {
		return json.NewEncoder(os.Stdout).Encode(device)
	}
	return printDeviceTable([]client.Device{device})
}

func printDeviceTable(items []client.Device) error {
	table, err := formatDeviceTable(items)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(os.Stdout, table)
	return err
}

func statusColumns(status *client.DeviceStatus) (phase, gateway, reachable, degraded string) {
	if status == nil {
		return "-", "-", "-", "-"
	}
	phase = status.Phase
	if phase == "" {
		phase = "-"
	}
	gateway = boolLabel(status.GatewayReady)
	reachable = boolLabel(status.DeviceReachable)
	degraded = boolLabel(status.Degraded)
	return phase, gateway, reachable, degraded
}

func boolLabel(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
