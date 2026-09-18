package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kalw/spm/internal/archive"
	"github.com/kalw/spm/internal/manifest"
)

// buildArtifact loads the manifest in dir, builds the tarball and sha256, and
// returns them along with the loaded manifest. Nothing is written to disk.
func buildArtifact(dir string) (*manifest.Manifest, []byte, string, error) {
	m, err := manifest.Load(dir)
	if err != nil {
		return nil, nil, "", err
	}
	tarball, sum, err := archive.BuildBytes(dir, m)
	if err != nil {
		return nil, nil, "", err
	}
	return m, tarball, sum, nil
}

// writeDist writes the tarball and its .sha256 sidecar into dir/dist.
func writeDist(dir string, m *manifest.Manifest, tarball []byte, sum string) (string, error) {
	distDir := filepath.Join(dir, "dist")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		return "", err
	}
	artifact := m.Artifact()
	tarPath := filepath.Join(distDir, artifact)
	if err := os.WriteFile(tarPath, tarball, 0o644); err != nil {
		return "", err
	}
	sumPath := tarPath + ".sha256"
	if err := os.WriteFile(sumPath, []byte(archive.ChecksumFile(sum, artifact)), 0o644); err != nil {
		return "", err
	}
	return tarPath, nil
}

func cmdPackage(args []string) error {
	dir := dirArg(args)
	m, tarball, sum, err := buildArtifact(dir)
	if err != nil {
		return err
	}
	tarPath, err := writeDist(dir, m, tarball, sum)
	if err != nil {
		return err
	}
	fmt.Printf("built %s\n  sha256:%s\n", tarPath, sum)
	return nil
}
