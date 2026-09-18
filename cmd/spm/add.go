package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kalw/spm/internal/config"
	"github.com/kalw/spm/internal/miseconf"
)

func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	remoteName := fs.String("from", "", "name of the remote the package lives on (required)")
	write := fs.Bool("write", false, "append the snippet to ./mise.toml instead of printing")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if *remoteName == "" {
		return fmt.Errorf("--from <remote> is required")
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: spm add <name>[@version] --from <remote>")
	}
	pkg, version := rest[0], "latest"
	if i := strings.IndexByte(pkg, '@'); i >= 0 {
		pkg, version = pkg[:i], pkg[i+1:]
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	remote, err := cfg.Remote(*remoteName)
	if err != nil {
		return err
	}
	snippet, err := miseconf.Snippet(remote, pkg, version)
	if err != nil {
		return err
	}

	if !*write {
		fmt.Print(snippet)
		return nil
	}
	f, err := os.OpenFile("mise.toml", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString("\n" + snippet); err != nil {
		return err
	}
	fmt.Printf("appended http/s3 tool %q to ./mise.toml\n", pkg)
	return nil
}
