package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/iruoy/fylla/internal/task"
)

// MoveParams holds inputs for the move command.
type MoveParams struct {
	TaskKey  string
	Target   string // empty = prompt user
	Provider string
	Source   TaskSource
	Surveyor Surveyor
}

// MoveResult holds the output of a move operation.
type MoveResult struct {
	TaskKey string
	Target  string
}

// RunMove transitions a task to a target status/lane.
func RunMove(ctx context.Context, p MoveParams) (*MoveResult, error) {
	key, _ := task.StripInstanceSuffix(p.TaskKey)

	// Route to provider
	var src interface{} = p.Source
	if p.Provider != "" {
		if ms, ok := p.Source.(*MultiTaskSource); ok {
			if routed, ok := ms.RouteToProvider(p.Provider); ok {
				src = routed
			}
		}
	}

	lister, ok := src.(TransitionLister)
	if !ok {
		return nil, fmt.Errorf("provider for %q does not support transitions", key)
	}

	target := p.Target
	if target == "" {
		transitions, err := lister.ListTransitions(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("list transitions: %w", err)
		}
		if len(transitions) == 0 {
			return nil, fmt.Errorf("no transitions available for %s", key)
		}
		if p.Surveyor == nil {
			return nil, fmt.Errorf("no target specified and no interactive prompt available")
		}
		selected, err := p.Surveyor.Select("Move to:", transitions)
		if err != nil {
			return nil, fmt.Errorf("select transition: %w", err)
		}
		target = selected
	}

	transitioner, ok := src.(Transitioner)
	if !ok {
		return nil, fmt.Errorf("provider for %q does not support transitions", key)
	}

	if err := transitioner.TransitionTask(ctx, key, target); err != nil {
		return nil, fmt.Errorf("move task: %w", err)
	}

	return &MoveResult{TaskKey: p.TaskKey, Target: target}, nil
}

// PrintMoveResult writes the move confirmation to the given writer.
func PrintMoveResult(w io.Writer, result *MoveResult) {
	fmt.Fprintf(w, "Moved %s to %s\n", result.TaskKey, result.Target)
}
