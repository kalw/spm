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
