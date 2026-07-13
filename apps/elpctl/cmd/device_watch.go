package cmd

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/trains-io/elp/apps/elpctl/internal/client"
)

type deviceWatchWriter struct {
	out    io.Writer
	isTTY  func(io.Writer) bool
	lines  int
	byName map[string]client.Device
}

func newDeviceWatchWriter(out io.Writer) *deviceWatchWriter {
	return &deviceWatchWriter{
		out:    out,
		isTTY:  writerIsTerminal,
		byName: make(map[string]client.Device),
	}
}

func (w *deviceWatchWriter) writeSnapshot(items []client.Device) error {
	w.byName = make(map[string]client.Device, len(items))
	for _, item := range items {
		w.byName[deviceMapKey(item)] = item
	}
	return w.render()
}

func (w *deviceWatchWriter) writeUpdated(device client.Device) error {
	if w.byName == nil {
		w.byName = make(map[string]client.Device)
	}
	w.byName[deviceMapKey(device)] = device
	return w.render()
}

func (w *deviceWatchWriter) writeDeleted(name string) error {
	for key, device := range w.byName {
		if device.Name == name {
			delete(w.byName, key)
			break
		}
	}
	if err := w.render(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w.out, "deleted\t%s\n", name)
	return err
}

func (w *deviceWatchWriter) render() error {
	table, err := formatDeviceTable(devicesFromMap(w.byName))
	if err != nil {
		return err
	}
	redraw := w.isTTY(w.out) && w.lines > 0
	if redraw {
		if _, err := fmt.Fprint(w.out, strings.Repeat("\033[F", w.lines)); err != nil {
			return err
		}
	}
	w.lines = deviceTableLineCount(table)
	output := table
	if redraw {
		output = eraseToEndOfLine(table)
	}
	_, err = fmt.Fprint(w.out, output)
	return err
}

// eraseToEndOfLine appends ANSI EL to each row so shorter redraws do not leave
// trailing characters from the previous table (tabwriter column widths can shrink).
func eraseToEndOfLine(table string) string {
	table = strings.TrimRight(table, "\n")
	if table == "" {
		return ""
	}
	lines := strings.Split(table, "\n")
	for i, line := range lines {
		lines[i] = line + "\033[K"
	}
	return strings.Join(lines, "\n") + "\n"
}

func devicesFromMap(byName map[string]client.Device) []client.Device {
	items := make([]client.Device, 0, len(byName))
	for _, device := range byName {
		items = append(items, device)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Namespace != items[j].Namespace {
			return items[i].Namespace < items[j].Namespace
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func deviceMapKey(device client.Device) string {
	return device.Namespace + "/" + device.Name
}

func formatDeviceTable(items []client.Device) (string, error) {
	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tNAMESPACE\tADDRESS\tPHASE\tGATEWAY\tREACHABLE\tDEGRADED")
	for _, d := range items {
		phase, gateway, reachable, degraded := statusColumns(d.Status)
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			d.Name, d.Namespace, d.Address, phase, gateway, reachable, degraded)
	}
	if err := tw.Flush(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func deviceTableLineCount(table string) int {
	table = strings.TrimRight(table, "\n")
	if table == "" {
		return 0
	}
	return strings.Count(table, "\n") + 1
}

func writerIsTerminal(out io.Writer) bool {
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
