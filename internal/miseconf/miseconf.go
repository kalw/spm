// Package miseconf renders the native mise [tools] snippet a consumer adds to
// their mise.toml to install a package from a given remote.
package miseconf

import (
	"fmt"
	"strings"

	"github.com/kalw/spm/internal/config"
)

// Snippet returns the mise.toml [tools] block for installing pkg at version
// (which may be "latest") from remote r.
func Snippet(r config.Remote, pkg, version string) (string, error) {
	switch r.Type {
	case config.TypeHTTP:
		return httpSnippet(strings.TrimRight(r.DownloadBaseURL, "/"), pkg, version), nil
	case config.TypeS3:
		return s3Snippet(r, pkg, version, ""), nil
	case config.TypeGCS:
		if r.Access == config.GCSInterop {
			return s3Snippet(r, pkg, version, config.GCSInteropEndpoint), nil
		}
		// native access → consumers fetch over HTTPS via the http backend.
		if r.PublicBaseURL == "" {
			return "", fmt.Errorf("remote %q: gcs native access needs public_base_url for consumer config", r.Name())
		}
		return httpSnippet(strings.TrimRight(r.PublicBaseURL, "/"), pkg, version), nil
	default:
		return "", fmt.Errorf("unsupported remote type %q", r.Type)
	}
}

func httpSnippet(base, pkg, version string) string {
	dir := base + "/" + pkg
	var b strings.Builder
	fmt.Fprintf(&b, "[tools.\"http:%s\"]\n", pkg)
	fmt.Fprintf(&b, "version = %q\n", version)
	fmt.Fprintf(&b, "url = \"%s/%s-{{version}}.tar.gz\"\n", dir, pkg)
	fmt.Fprintf(&b, "checksum_url = \"%s/%s-{{version}}.tar.gz.sha256\"\n", dir, pkg)
	fmt.Fprintf(&b, "version_list_url = \"%s/versions.json\"\n", dir)
	b.WriteString("version_order = \"semver\"\n")
	return b.String()
}

func s3Snippet(r config.Remote, pkg, version, endpoint string) string {
	dir := fmt.Sprintf("s3://%s/%s", r.Bucket, strings.Trim(r.Key(pkg, ""), "/"))
	var b strings.Builder
	fmt.Fprintf(&b, "[tools.\"s3:%s\"]\n", pkg)
	fmt.Fprintf(&b, "version = %q\n", version)
	fmt.Fprintf(&b, "url = \"%s/%s-{{version}}.tar.gz\"\n", dir, pkg)
	fmt.Fprintf(&b, "version_list_url = \"%s/versions.json\"\n", dir)
	b.WriteString("version_order = \"semver\"\n")
	if r.Region != "" {
		fmt.Fprintf(&b, "region = %q\n", r.Region)
	}
	if endpoint != "" {
		fmt.Fprintf(&b, "endpoint = %q\n", endpoint)
	} else if r.Endpoint != "" {
		fmt.Fprintf(&b, "endpoint = %q\n", r.Endpoint)
	}
	b.WriteString("# checksum is recorded and verified via mise.lock (run `mise install` then commit mise.lock)\n")
	return b.String()
}
