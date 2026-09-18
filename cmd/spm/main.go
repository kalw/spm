// Command spm packages shell scripts and publishes them to private storage
// (HTTP, S3, GCS) for consumption by mise.
package main

import (
	"flag"
	"fmt"
	"os"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

const usage = `spm - package shell scripts for mise + private storage

Usage:
  spm init [dir]                     scaffold a new package (spm.toml + src/)
  spm package [dir]                  build dist/<name>-<version>.tar.gz + .sha256
  spm publish [dir] --to <remote>    package and upload to a configured remote
  spm list --from <remote> <name>    list published versions of a package
  spm add <name>[@version] --from <remote> [--write]
                                     print the mise.toml snippet for a package
  spm version                        print the spm version

Remotes are configured in ` + "`$SPM_CONFIG`" + ` (default ~/.config/spm/config.toml).
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = cmdInit(os.Args[2:])
	case "package":
		err = cmdPackage(os.Args[2:])
	case "publish":
		err = cmdPublish(os.Args[2:])
	case "list":
		err = cmdList(os.Args[2:])
	case "add":
		err = cmdAdd(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("spm %s\n", version)
		return
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// dirArg returns the first positional as a directory, defaulting to ".".
func dirArg(args []string) string {
	for _, a := range args {
		if len(a) > 0 && a[0] != '-' {
			return a
		}
	}
	return "."
}

// parseFlags parses fs allowing flags and positionals to interleave, which the
// stdlib flag package does not do on its own (it stops at the first positional).
// It returns the collected positional arguments.
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var positionals []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			break
		}
		positionals = append(positionals, args[0])
		args = args[1:]
	}
	return positionals, nil
}
