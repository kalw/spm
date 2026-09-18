package miseconf

import (
	"strings"
	"testing"

	"github.com/kalw/spm/internal/config"
	"github.com/kalw/spm/internal/manifest"
)

func TestHTTPSnippet(t *testing.T) {
	r := config.Remote{
		Type:            config.TypeHTTP,
		DownloadBaseURL: "https://dl.acme.com/spm/",
	}
	got, err := Snippet(r, "deploy-tools", "1.4.2")
	if err != nil {
		t.Fatal(err)
	}
	want := `[tools."http:deploy-tools"]
version = "1.4.2"
url = "https://dl.acme.com/spm/deploy-tools/deploy-tools-{{version}}.tar.gz"
checksum_url = "https://dl.acme.com/spm/deploy-tools/deploy-tools-{{version}}.tar.gz.sha256"
version_list_url = "https://dl.acme.com/spm/deploy-tools/versions.json"
version_order = "semver"
`
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestS3Snippet(t *testing.T) {
	r := config.Remote{
		Type:   config.TypeS3,
		Bucket: "acme-artifacts",
		Prefix: "spm",
		Region: "us-east-1",
	}
	got, err := Snippet(r, "deploy-tools", "latest")
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{
		`[tools."s3:deploy-tools"]`,
		`version = "latest"`,
		`url = "s3://acme-artifacts/spm/deploy-tools/deploy-tools-{{version}}.tar.gz"`,
		`version_list_url = "s3://acme-artifacts/spm/deploy-tools/versions.json"`,
		`region = "us-east-1"`,
		`version_order = "semver"`,
	} {
		if !strings.Contains(got, sub) {
			t.Fatalf("s3 snippet missing %q in:\n%s", sub, got)
		}
	}
}

func TestGCSInteropSnippetUsesEndpoint(t *testing.T) {
	r := config.Remote{
		Type:   config.TypeGCS,
		Bucket: "acme-artifacts",
		Prefix: "spm",
		Access: config.GCSInterop,
	}
	got, err := Snippet(r, "deploy-tools", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `[tools."s3:deploy-tools"]`) {
		t.Fatalf("gcs interop should use s3 backend:\n%s", got)
	}
	if !strings.Contains(got, `endpoint = "https://storage.googleapis.com"`) {
		t.Fatalf("gcs interop should set interop endpoint:\n%s", got)
	}
}

func TestGCSNativeSnippetUsesHTTP(t *testing.T) {
	r := config.Remote{
		Type:          config.TypeGCS,
		Bucket:        "acme-artifacts",
		Prefix:        "spm",
		Access:        config.GCSNative,
		PublicBaseURL: "https://storage.googleapis.com/acme-artifacts/spm",
	}
	got, err := Snippet(r, "deploy-tools", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `[tools."http:deploy-tools"]`) {
		t.Fatalf("gcs native should use http backend:\n%s", got)
	}
}

func TestDepBlocks(t *testing.T) {
	deps := []manifest.Dep{
		{Mise: "jq", Version: "1.7"},
		{Mise: "npm:cowsay"},
	}
	got := DepBlocks(deps)
	want := "\n[tools.\"jq\"]\nversion = \"1.7\"\n\n[tools.\"npm:cowsay\"]\nversion = \"latest\"\n"
	if got != want {
		t.Fatalf("DepBlocks got:\n%q\nwant:\n%q", got, want)
	}
	if DepBlocks(nil) != "" {
		t.Fatal("no deps should render empty")
	}
}

func TestDependsLine(t *testing.T) {
	deps := []manifest.Dep{{Mise: "jq", Version: "1.7"}, {Mise: "npm:cowsay"}}
	if got := DependsLine(deps); got != "depends = [\"jq\", \"npm:cowsay\"]\n" {
		t.Fatalf("DependsLine = %q", got)
	}
	if DependsLine(nil) != "" {
		t.Fatal("no deps should render empty depends")
	}
}
