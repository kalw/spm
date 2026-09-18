package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func writePkg(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "deploy.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadValid(t *testing.T) {
	dir := writePkg(t, `name = "deploy-tools"
version = "1.4.2"
[[bin]]
name = "deploy"
path = "src/deploy.sh"
`)
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Artifact(); got != "deploy-tools-1.4.2.tar.gz" {
		t.Fatalf("artifact = %q", got)
	}
}

func TestRejectBadVersion(t *testing.T) {
	for _, v := range []string{"1.4", "v1.4.2", "1.4.x", "latest"} {
		dir := writePkg(t, `name = "x"
version = "`+v+`"
[[bin]]
name = "deploy"
path = "src/deploy.sh"
`)
		if _, err := Load(dir); err == nil {
			t.Fatalf("version %q should be rejected", v)
		}
	}
}

func TestRejectBadName(t *testing.T) {
	dir := writePkg(t, `name = "Deploy_Tools"
version = "1.0.0"
[[bin]]
name = "deploy"
path = "src/deploy.sh"
`)
	if _, err := Load(dir); err == nil {
		t.Fatal("uppercase/underscore name should be rejected")
	}
}

func TestRejectMissingBinSource(t *testing.T) {
	dir := writePkg(t, `name = "x"
version = "1.0.0"
[[bin]]
name = "gone"
path = "src/missing.sh"
`)
	if _, err := Load(dir); err == nil {
		t.Fatal("missing bin source should be rejected")
	}
}
