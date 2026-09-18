package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/kalw/spm/internal/archive"
	"github.com/kalw/spm/internal/config"
	"github.com/kalw/spm/internal/meta"
	"github.com/kalw/spm/internal/storage"
	"github.com/kalw/spm/internal/versions"
)

func cmdPublish(args []string) error {
	fs := flag.NewFlagSet("publish", flag.ContinueOnError)
	remoteName := fs.String("to", "", "name of the remote to publish to (required)")
	force := fs.Bool("force", false, "overwrite an already-published version")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if *remoteName == "" {
		return fmt.Errorf("--to <remote> is required")
	}
	dir := dirArg(pos)

	m, tarball, sum, err := buildArtifact(dir)
	if err != nil {
		return err
	}

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

	artifact := m.Artifact()
	tarKey := remote.Key(m.Name, artifact)

	// Immutability: refuse to clobber an existing artifact unless forced.
	if !*force {
		exists, err := store.Exists(ctx, tarKey)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("%s already published to %s (use --force to overwrite)", artifact, remote.Name())
		}
	}

	// Read-merge-write the version manifest first so we fail before uploading if
	// the version is a duplicate.
	versKey := remote.Key(m.Name, "versions.json")
	body, _, err := store.Get(ctx, versKey)
	if err != nil {
		return err
	}
	list, err := versions.Parse(body)
	if err != nil {
		return err
	}
	merged, err := versions.Merge(list, m.Version, *force)
	if err != nil {
		return err
	}
	versJSON, err := versions.Encode(merged)
	if err != nil {
		return err
	}

	// Upload artifact, checksum, then the updated manifest last.
	if err := store.Put(ctx, tarKey, tarball, "application/gzip"); err != nil {
		return err
	}
	sumBody := []byte(archive.ChecksumFile(sum, artifact))
	if err := store.Put(ctx, tarKey+".sha256", sumBody, "text/plain"); err != nil {
		return err
	}

	// Metadata sidecar: lets consumers discover this version's deps without
	// downloading the tarball.
	metaBody, err := meta.Encode(meta.Meta{Name: m.Name, Version: m.Version, Deps: m.Deps})
	if err != nil {
		return err
	}
	if err := store.Put(ctx, remote.Key(m.Name, meta.FileName(m.Name, m.Version)), metaBody, "application/json"); err != nil {
		return err
	}

	if err := store.Put(ctx, versKey, versJSON, "application/json"); err != nil {
		return err
	}

	fmt.Printf("published %s to %s\n  key: %s\n  sha256:%s\n  versions: %v\n",
		artifact, remote.Name(), tarKey, sum, merged)
	return nil
}
