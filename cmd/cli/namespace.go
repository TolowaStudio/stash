package main

import (
	"context"
	"fmt"

	"github.com/alash3al/stash/internal/brain"
	"github.com/urfave/cli/v3"
)

func namespaceCreateCmd(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args()
	if args.Len() == 0 {
		return fmt.Errorf("namespace slug is required")
	}
	slug := args.First()
	name := cmd.String("name")
	if name == "" {
		name = slug
	}
	description := cmd.String("description")

	bc := getBootstrap(cmd)
	id, err := bc.Brain.CreateNamespace(ctx, slug, name, description)
	if err != nil {
		return err
	}
	return printJSON(map[string]any{"id": id, "slug": slug})
}

func namespaceListCmd(ctx context.Context, cmd *cli.Command) error {
	patterns := cmd.StringSlice("namespaces")
	page := brain.Pagination{
		Offset: cmd.Int("offset"),
		Limit:  cmd.Int("limit"),
	}
	bc := getBootstrap(cmd)
	namespaces, err := bc.Brain.ListNamespaces(ctx, patterns, page)
	if err != nil {
		return err
	}
	return printJSON(namespaces)
}

func namespaceDescribeCmd(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args()
	if args.Len() == 0 {
		return fmt.Errorf("namespace slug is required")
	}
	slug := args.First()
	bc := getBootstrap(cmd)
	if err := bc.Brain.UpdateNamespaceDescription(ctx, slug, cmd.String("description")); err != nil {
		return err
	}
	return printJSON(map[string]any{"slug": slug, "description": cmd.String("description")})
}

func namespaceRenameCmd(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args()
	if args.Len() != 2 {
		return fmt.Errorf("usage: namespace rename <from-slug> <to-slug>")
	}
	from, to := args.Get(0), args.Get(1)
	bc := getBootstrap(cmd)
	if err := bc.Brain.RenameNamespace(ctx, from, to); err != nil {
		return err
	}
	return printJSON(map[string]any{"from": from, "to": to})
}

func namespaceMergeCmd(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args()
	if args.Len() != 2 {
		return fmt.Errorf("usage: namespace merge <source-slug> <target-slug>")
	}
	source, target := args.Get(0), args.Get(1)
	bc := getBootstrap(cmd)
	res, err := bc.Brain.MergeNamespaces(ctx, source, target)
	if err != nil {
		return err
	}
	return printJSON(map[string]any{"source": source, "target": target, "result": res})
}
