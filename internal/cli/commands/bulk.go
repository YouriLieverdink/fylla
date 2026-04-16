package commands

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// BulkAction defines the type of bulk operation.
type BulkAction string

const (
	BulkDone   BulkAction = "done"
	BulkDelete BulkAction = "delete"
)

// BulkParams holds inputs for a bulk operation.
type BulkParams struct {
	Action   BulkAction
	TaskKeys []string // keys of tasks to operate on
	Provider string   // optional provider hint
	Source   TaskSource
}

// BulkResult holds the output of a bulk operation.
type BulkResult struct {
	Action    BulkAction
	Succeeded []string
	Failed    map[string]error
}

// RunBulk executes a bulk operation on multiple tasks.
func RunBulk(ctx context.Context, p BulkParams) (*BulkResult, error) {
	if len(p.TaskKeys) == 0 {
		return nil, fmt.Errorf("no tasks selected")
	}

	result := &BulkResult{
		Action: p.Action,
		Failed: make(map[string]error),
	}

	for _, key := range p.TaskKeys {
		src := routedSourceFor(p.Source, key, p.Provider)
		var err error
		switch p.Action {
		case BulkDone:
			err = src.CompleteTask(ctx, key)
		case BulkDelete:
			err = src.DeleteTask(ctx, key)
		default:
			err = fmt.Errorf("unknown action %q", p.Action)
		}

		if err != nil {
			result.Failed[key] = err
		} else {
			result.Succeeded = append(result.Succeeded, key)
		}
	}

	return result, nil
}

// PrintBulkResult writes the bulk operation result to the given writer.
func PrintBulkResult(w io.Writer, result *BulkResult) {
	action := string(result.Action)
	actionTitle := strings.ToUpper(action[:1]) + action[1:]
	if len(result.Succeeded) > 0 {
		fmt.Fprintf(w, "%s %d task(s): %s\n",
			actionTitle,
			len(result.Succeeded),
			strings.Join(result.Succeeded, ", "))
	}
	for key, err := range result.Failed {
		fmt.Fprintf(w, "Failed to %s %s: %v\n", action, key, err)
	}
}
