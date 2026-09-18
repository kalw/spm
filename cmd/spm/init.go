package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kalw/spm/internal/manifest"
)

const sampleManifest = `name = "example-tools"
version = "0.1.0"
description = "Example shell package"

[[bin]]
name = "hello"
path = "src/hello.sh"

# Runtime dependencies provided by mise. Each is emitted into the consumer's
# mise.toml by ` + "`spm add`" + ` and checked by a preflight injected into shell
# scripts (resolved via ` + "`mise which`" + `, falling back to PATH).
# [[deps]]
# mise = "jq"        # mise tool ref (registry name or backend ref like npm:cowsay)
# version = "1.7"    # optional; defaults to "latest"
# bin = "jq"         # optional; command checked at preflight (derived if omitted)
`

const sampleScript = `#!/usr/bin/env bash
set -euo pipefail
echo "hello from spm"
`

func cmdInit(args []string) error {
	dir := dirArg(args)
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		return err
	}
	mf := filepath.Join(dir, manifest.FileName)
	if _, err := os.Stat(mf); err == nil {
		return fmt.Errorf("%s already exists", mf)
	}
	if err := os.WriteFile(mf, []byte(sampleManifest), 0o644); err != nil {
		return err
	}
	script := filepath.Join(dir, "src", "hello.sh")
	if _, err := os.Stat(script); os.IsNotExist(err) {
		if err := os.WriteFile(script, []byte(sampleScript), 0o755); err != nil {
			return err
		}
	}
	fmt.Printf("scaffolded package in %s\n  edit %s, then run: spm package %s\n", dir, mf, dir)
	return nil
}
