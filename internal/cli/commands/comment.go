package commands

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/iruoy/fylla/internal/timer"
	"github.com/spf13/cobra"
)

func newCommentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "comment [text]",
		Short: "Add a comment to the running timer",
		RunE: func(cmd *cobra.Command, args []string) error {
			comment := strings.Join(args, " ")
			if comment == "" {
				prompt := &survey.Input{Message: "Comment:"}
				if err := survey.AskOne(prompt, &comment); err != nil {
					return fmt.Errorf("prompt comment: %w", err)
				}
			}

			path, err := timer.DefaultPath()
			if err != nil {
				return fmt.Errorf("timer path: %w", err)
			}
			if err := timer.SetComment(comment, path); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Comment set: %s\n", comment)
			return nil
		},
	}
}
