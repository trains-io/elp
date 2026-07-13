package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type usageError struct {
	cmd    *cobra.Command
	reason string
}

func (e *usageError) Error() string {
	if e.reason != "" {
		return e.reason
	}
	return "invalid command usage"
}

func usageErr(cmd *cobra.Command, reason string) error {
	return &usageError{cmd: cmd, reason: reason}
}

func wrapArgs(validator cobra.PositionalArgs) cobra.PositionalArgs {
	if validator == nil {
		return nil
	}
	return func(cmd *cobra.Command, args []string) error {
		if err := validator(cmd, args); err != nil {
			return &usageError{cmd: cmd, reason: err.Error()}
		}
		return nil
	}
}

// NoArgs rejects unexpected positional arguments and shows command help on failure.
func NoArgs() cobra.PositionalArgs {
	return wrapArgs(cobra.NoArgs)
}

// ExactArgs requires exactly n positional arguments and shows command help on failure.
func ExactArgs(n int) cobra.PositionalArgs {
	return wrapArgs(cobra.ExactArgs(n))
}

// MaximumNArgs limits positional arguments and shows command help on failure.
func MaximumNArgs(n int) cobra.PositionalArgs {
	return wrapArgs(cobra.MaximumNArgs(n))
}

func setUsageOnError(cmd *cobra.Command) {
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return &usageError{cmd: c, reason: err.Error()}
	})
	for _, sub := range cmd.Commands() {
		setUsageOnError(sub)
	}
}

func printUsageError(err error) bool {
	var usageErr *usageError
	if !errors.As(err, &usageErr) {
		return false
	}

	fmt.Fprintf(os.Stderr, "error: %s\n\n", usageErr.Error())
	_ = usageErr.cmd.Help()
	return true
}
