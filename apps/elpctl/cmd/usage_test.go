package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func captureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	origStdout := os.Stdout
	origStderr := os.Stderr
	os.Stdout = stdoutW
	os.Stderr = stderrW

	fn()

	stdoutW.Close()
	stderrW.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	var stdoutBuf, stderrBuf bytes.Buffer
	if _, err := io.Copy(&stdoutBuf, stdoutR); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(&stderrBuf, stderrR); err != nil {
		t.Fatal(err)
	}
	return stdoutBuf.String(), stderrBuf.String()
}

func TestDeviceCreateMissingNameShowsHelp(t *testing.T) {
	stdout, stderr := captureOutput(t, func() {
		rootCmd.SetArgs([]string{"device", "create", "--server", "http://127.0.0.1:1"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error")
		}
		if !printUsageError(err) {
			t.Fatalf("expected usageError, got %T: %v", err, err)
		}
	})

	if !strings.Contains(stderr, "error: accepts 1 arg(s), received 0") {
		t.Fatalf("stderr missing arg error:\n%s", stderr)
	}
	if !strings.Contains(stdout, "create NAME") {
		t.Fatalf("stdout missing create subcommand help:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--address") {
		t.Fatalf("stdout missing create flags:\n%s", stdout)
	}
}

func TestDeviceCreateHardwareMissingAddressShowsHelp(t *testing.T) {
	stdout, stderr := captureOutput(t, func() {
		rootCmd.SetArgs([]string{
			"device", "create", "bench",
			"--backend", "hardware",
			"--server", "http://127.0.0.1:1",
		})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error")
		}
		if !printUsageError(err) {
			t.Fatalf("expected usageError, got %T: %v", err, err)
		}
	})

	if !strings.Contains(stderr, "error: --address is required for hardware backend") {
		t.Fatalf("stderr missing validation error:\n%s", stderr)
	}
	if !strings.Contains(stdout, "create NAME") {
		t.Fatalf("stdout missing create subcommand help:\n%s", stdout)
	}
}

func TestDeviceCreateUnsupportedBackendShowsHelp(t *testing.T) {
	stdout, stderr := captureOutput(t, func() {
		rootCmd.SetArgs([]string{
			"device", "create", "bench",
			"--backend", "cloud",
			"--server", "http://127.0.0.1:1",
		})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error")
		}
		if !printUsageError(err) {
			t.Fatalf("expected usageError, got %T: %v", err, err)
		}
	})

	if !strings.Contains(stderr, `error: unsupported backend type "cloud"`) {
		t.Fatalf("stderr missing backend error:\n%s", stderr)
	}
	if !strings.Contains(stdout, "--backend") {
		t.Fatalf("stdout missing create flags:\n%s", stdout)
	}
}
