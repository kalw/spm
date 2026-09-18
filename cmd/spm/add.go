package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/kalw/spm/internal/config"
	"github.com/kalw/spm/internal/manifest"
	"github.com/kalw/spm/internal/meta"
	"github.com/kalw/spm/internal/miseconf"
	"github.com/kalw/spm/internal/storage"
	"github.com/kalw/spm/internal/versions"
)

func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	remoteName := fs.String("from", "", "name of the remote the package lives on (required)")
	write := fs.Bool("write", false, "append the snippet to ./mise.toml instead of printing")
	noDeps := fs.Bool("no-deps", false, "do not emit dependency tool blocks")
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

	// Resolve the package's dependencies from its metadata sidecar and append
	// them as mise tool blocks. Best-effort: a missing sidecar (older package
	// or unreachable store) just omits the dep blocks with a note on stderr.
	if !*noDeps {
		if deps, derr := resolveDeps(remote, pkg, version); derr != nil {
			fmt.Fprintf(os.Stderr, "spm: could not resolve dependencies for %s: %v\n", pkg, derr)
		} else if blocks := miseconf.DepBlocks(deps); blocks != "" {
			snippet += blocks
		}
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
	fmt.Printf("appended tool %q (and its deps) to ./mise.toml\n", pkg)
	return nil
}

// resolveDeps fetches the metadata sidecar for the requested version and returns
// its declared dependencies.
func resolveDeps(remote config.Remote, pkg, version string) ([]manifest.Dep, error) {
	ctx := context.Background()
	concrete, err := concreteVersion(ctx, remote, pkg, version)
	if err != nil {
		return nil, err
	}
	body, found, err := consumerGet(ctx, remote, pkg, meta.FileName(pkg, concrete))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil // no sidecar published for this version
	}
	m, err := meta.Parse(body)
	if err != nil {
		return nil, err
	}
	return m.Deps, nil
}

// concreteVersion returns a specific version string to locate the sidecar: the
// request itself when it is exact semver, otherwise the latest published one.
func concreteVersion(ctx context.Context, remote config.Remote, pkg, version string) (string, error) {
	if _, err := semver.StrictNewVersion(version); err == nil {
		return version, nil
	}
	body, found, err := consumerGet(ctx, remote, pkg, "versions.json")
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("versions.json not found")
	}
	list, err := versions.Parse(body)
	if err != nil {
		return "", err
	}
	sorted, err := versions.Sort(list)
	if err != nil {
		return "", err
	}
	if len(sorted) == 0 {
		return "", fmt.Errorf("no published versions")
	}
	return sorted[len(sorted)-1], nil
}

// consumerGet fetches a package file the way a consumer would: over HTTPS for
// http / gcs-native remotes, or through the object store for s3 / gcs-interop.
func consumerGet(ctx context.Context, remote config.Remote, pkg, file string) ([]byte, bool, error) {
	switch remote.Type {
	case config.TypeHTTP:
		return httpGet(ctx, strings.TrimRight(remote.DownloadBaseURL, "/")+"/"+pkg+"/"+file)
	case config.TypeGCS:
		if remote.Access == config.GCSNative {
			return httpGet(ctx, strings.TrimRight(remote.PublicBaseURL, "/")+"/"+pkg+"/"+file)
		}
		fallthrough
	case config.TypeS3:
		store, err := storage.New(ctx, remote)
		if err != nil {
			return nil, false, err
		}
		return store.Get(ctx, remote.Key(pkg, file))
	default:
		return nil, false, fmt.Errorf("unsupported remote type %q", remote.Type)
	}
}

func httpGet(ctx context.Context, url string) ([]byte, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}
