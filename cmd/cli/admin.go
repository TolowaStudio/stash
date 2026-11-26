package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func adminMergeNamespaceCmd(ctx context.Context, cmd *cli.Command) error {
	from := cmd.String("from")
	into := cmd.String("into")
	if from == "" || into == "" {
		return fmt.Errorf("--from and --into are required")
	}

	dryRun := cmd.Bool("dry-run")
	bc := getBootstrap(cmd)
	result, err := bc.Brain.MergeNamespace(ctx, from, into, dryRun)
	if err != nil {
		return err
	}
	return printJSON(result)
}