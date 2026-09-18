package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kalw/spm/internal/manifest"
)

func fixture(t *testing.T) (string, *manifest.Manifest) {
	t.Helper()
	dir := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(dir, "src"), 0o755))
	must(os.WriteFile(filepath.Join(dir, "src", "a.sh"), []byte("echo a\n"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "src", "b.sh"), []byte("echo b\n"), 0o644))
	m := &manifest.Manifest{
		Name:    "tools",
		Version: "1.0.0",
		Bin: []manifest.Bin{
			{Name: "beta", Path: "src/b.sh"},
			{Name: "alpha", Path: "src/a.sh"},
		},
	}
	return dir, m
}

func TestReproducible(t *testing.T) {
	dir, m := fixture(t)
	_, sum1, err := BuildBytes(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	b2, sum2, err := BuildBytes(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	if sum1 != sum2 {
		t.Fatalf("sha256 not reproducible: %s vs %s", sum1, sum2)
	}
	if len(sum1) != 64 {
		t.Fatalf("sha256 hex length = %d", len(sum1))
	}
	_ = b2
}

func TestLayoutSortedAndExecutable(t *testing.T) {
	dir, m := fixture(t)
	data, _, err := BuildBytes(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
		if h.Mode != 0o755 {
			t.Fatalf("%s mode = %o, want 0755", h.Name, h.Mode)
		}
	}
	want := []string{"bin/alpha", "bin/beta"} // sorted, remapped to bin/<name>
	if len(names) != len(want) {
		t.Fatalf("entries = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("entry %d = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestChecksumFile(t *testing.T) {
	got := ChecksumFile("abc123", "tools-1.0.0.tar.gz")
	want := "abc123  tools-1.0.0.tar.gz\n"
	if got != want {
		t.Fatalf("ChecksumFile = %q, want %q", got, want)
	}
}
