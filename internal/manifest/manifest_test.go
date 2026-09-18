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

func TestDepsValidation(t *testing.T) {
	base := `name = "x"
version = "1.0.0"
[[bin]]
name = "deploy"
path = "src/deploy.sh"
`
	// valid deps
	dir := writePkg(t, base+`
[[deps]]
mise = "jq"
version = "1.7"
[[deps]]
mise = "npm:cowsay"
`)
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Deps) != 2 || !m.PreflightEnabled() {
		t.Fatalf("deps=%v preflight=%v", m.Deps, m.PreflightEnabled())
	}
	if got := m.Deps[0].EffectiveBin(); got != "jq" {
		t.Fatalf("bin derive jq = %q", got)
	}
	if got := m.Deps[1].EffectiveBin(); got != "cowsay" {
		t.Fatalf("bin derive npm:cowsay = %q", got)
	}
	if got := m.Deps[0].MiseRef(); got != "jq@1.7" {
		t.Fatalf("MiseRef = %q", got)
	}
	if got := m.Deps[1].MiseRef(); got != "npm:cowsay" {
		t.Fatalf("MiseRef latest = %q", got)
	}

	// version in ref is rejected
	if _, err := Load(writePkg(t, base+"\n[[deps]]\nmise = \"jq@1.7\"\n")); err == nil {
		t.Fatal("dep ref with @version should be rejected")
	}
	// duplicate dep rejected
	if _, err := Load(writePkg(t, base+"\n[[deps]]\nmise = \"jq\"\n[[deps]]\nmise = \"jq\"\n")); err == nil {
		t.Fatal("duplicate dep should be rejected")
	}
}

func TestPreflightDisabled(t *testing.T) {
	dir := writePkg(t, `name = "x"
version = "1.0.0"
preflight = false
[[bin]]
name = "deploy"
path = "src/deploy.sh"
[[deps]]
mise = "jq"
`)
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.PreflightEnabled() {
		t.Fatal("preflight = false should disable")
	}
}
