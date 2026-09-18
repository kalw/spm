// Package archive builds a reproducible tar.gz for a package and its checksum.
package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kalw/spm/internal/manifest"
)

// epoch is a fixed mtime so identical inputs yield an identical archive (and sha256).
var epoch = time.Unix(0, 0)

// Build writes a deterministic tar.gz of the package in dir into out (a writer),
// laying each declared bin out at bin/<name> with mode 0755. It returns the
// lowercase hex sha256 of the compressed bytes.
func Build(dir string, m *manifest.Manifest, out io.Writer) (string, error) {
	// Assemble entries first so we can sort them for a stable layout.
	type entry struct {
		name string // path inside the tar, e.g. bin/deploy
		data []byte
	}
	var entries []entry
	for _, b := range m.Bin {
		data, err := os.ReadFile(filepath.Join(dir, b.Path))
		if err != nil {
			return "", fmt.Errorf("reading bin %q: %w", b.Name, err)
		}
		data = injectPreflight(data, m)
		entries = append(entries, entry{name: "bin/" + b.Name, data: data})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

	// Tee compressed output through a hasher so we get the sha256 for free.
	h := sha256.New()
	mw := io.MultiWriter(out, h)
	gz, _ := gzip.NewWriterLevel(mw, gzip.BestCompression)
	gz.ModTime = time.Time{} // omit gzip mtime header for reproducibility
	tw := tar.NewWriter(gz)

	for _, e := range entries {
		hdr := &tar.Header{
			Name:    e.name,
			Mode:    0o755,
			Size:    int64(len(e.data)),
			ModTime: epoch,
			Uid:     0,
			Gid:     0,
			Format:  tar.FormatGNU,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return "", err
		}
		if _, err := tw.Write(e.data); err != nil {
			return "", err
		}
	}
	if err := tw.Close(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// BuildBytes builds the archive fully in memory, returning the tarball and its sha256.
func BuildBytes(dir string, m *manifest.Manifest) ([]byte, string, error) {
	var buf bytes.Buffer
	sum, err := Build(dir, m, &buf)
	if err != nil {
		return nil, "", err
	}
	return buf.Bytes(), sum, nil
}

// ChecksumFile renders the sha256 sidecar contents ("<hex>  <artifact>\n"),
// matching the `sha256sum` format that mise's checksum_url understands.
func ChecksumFile(sum, artifact string) string {
	return fmt.Sprintf("%s  %s\n", sum, artifact)
}

// injectPreflight inserts a dependency-check prologue after the shebang of a
// shell script when the manifest declares deps and preflight is enabled. It is
// a no-op for scripts without a shell shebang, keeping non-shell bins untouched.
// The output is deterministic, so archive reproducibility is preserved.
func injectPreflight(data []byte, m *manifest.Manifest) []byte {
	if !m.PreflightEnabled() || len(m.Deps) == 0 {
		return data
	}
	nl := bytes.IndexByte(data, '\n')
	if nl < 0 {
		return data
	}
	first := data[:nl]
	if !bytes.HasPrefix(first, []byte("#!")) || !bytes.Contains(first, []byte("sh")) {
		return data
	}
	block := preflightBlock(m.Deps)
	out := make([]byte, 0, len(data)+len(block)+1)
	out = append(out, first...)
	out = append(out, '\n')
	out = append(out, block...)
	out = append(out, data[nl+1:]...)
	return out
}

// preflightBlock renders the POSIX-sh dependency check. It resolves each
// dependency via mise (`mise which`), falling back to PATH, and prints an
// actionable `mise use` hint when a dependency is missing.
func preflightBlock(deps []manifest.Dep) []byte {
	var b bytes.Buffer
	b.WriteString("# >>> spm preflight (auto-generated; do not edit) >>>\n")
	b.WriteString("__spm_require() {\n")
	b.WriteString("  command -v \"$1\" >/dev/null 2>&1 && return 0\n")
	b.WriteString("  command -v mise >/dev/null 2>&1 && mise which \"$1\" >/dev/null 2>&1 && return 0\n")
	b.WriteString("  printf 'spm: missing dependency: %s (install with: mise use %s)\\n' \"$1\" \"$2\" >&2\n")
	b.WriteString("  return 1\n")
	b.WriteString("}\n")
	for _, d := range deps {
		fmt.Fprintf(&b, "__spm_require %s %s || exit 1\n", shellSingleQuote(d.EffectiveBin()), shellSingleQuote(d.MiseRef()))
	}
	b.WriteString("unset -f __spm_require\n")
	b.WriteString("# <<< spm preflight <<<\n")
	return b.Bytes()
}

// shellSingleQuote wraps s in single quotes, safe for POSIX sh.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
