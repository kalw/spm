package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/kalw/spm/internal/config"
	"github.com/kalw/spm/internal/storage"
	"github.com/kalw/spm/internal/versions"
)

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	remoteName := fs.String("from", "", "name of the remote to read from (required)")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if *remoteName == "" {
		return fmt.Errorf("--from <remote> is required")
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: spm list --from <remote> <name>")
	}
	pkg := rest[0]

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	remote, err := cfg.Remote(*remoteName)
	if err != nil {
		return err
	}

	ctx := context.Background()
	store, err := storage.New(ctx, remote)
	if err != nil {
		return err
	}
	body, found, err := store.Get(ctx, remote.Key(pkg, "versions.json"))
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("package %q not found on %s", pkg, remote.Name())
	}
	list, err := versions.Parse(body)
	if err != nil {
		return err
	}
	sorted, err := versions.Sort(list)
	if err != nil {
		return err
	}
	for _, v := range sorted {
		fmt.Println(v)
	}
	return nil
}
