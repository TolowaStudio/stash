package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func adminExpireFactsCmd(ctx context.Context, cmd *cli.Command) error {
	namespace := cmd.String("namespace")
	if namespace == "" {
		return fmt.Errorf("--namespace is required")
	}

	match := cmd.String("match")
	apply := cmd.Bool("apply")
	dryRun := !apply

	bc := getBootstrap(cmd)
	result, err := bc.Brain.ExpireFacts(ctx, namespace, match, dryRun)
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Printf("matched %d fact(s) in %s (dry-run; pass --apply to set valid_until)\n", result.Matched, namespace)
	} else {
		fmt.Printf("expired %d of %d matched fact(s) in %s\n", result.Expired, result.Matched, namespace)
	}

	return printJSON(result)
}