package commands

import (
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/timer"
	"github.com/spf13/cobra"
)

// StartParams holds inputs for the start command.
type StartParams struct {
	TaskKey   string
	Project   string
	Section   string
	Provider  string
	TimerPath string
	Now       time.Time
}

// RunStart begins a timer for the specified task.
func RunStart(p StartParams) (*timer.State, error) {
	return timer.Start(p.TaskKey, p.Project, p.Section, p.Provider, p.Now, p.TimerPath)
}

// PrintStartResult writes the start confirmation to the given writer.
func PrintStartResult(w io.Writer, state *timer.State) {
	fmt.Fprintf(w, "Started timer for %s\n", state.TaskKey)
}

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start TASK-KEY",
		Short: "Start timer for a task",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := timer.DefaultPath()
			if err != nil {
				return fmt.Errorf("timer path: %w", err)
			}

			state, err := RunStart(StartParams{
				TaskKey:   args[0],
				TimerPath: path,
				Now:       time.Now(),
			})
			if err != nil {
				return err
			}

			PrintStartResult(cmd.OutOrStdout(), state)
			return nil
		},
	}
}
