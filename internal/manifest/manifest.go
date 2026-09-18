// Package manifest parses and validates the spm.toml package manifest.
package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/BurntSushi/toml"
	"github.com/Masterminds/semver/v3"
)

// FileName is the manifest file expected at the root of a package directory.
const FileName = "spm.toml"

var nameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// Bin maps an exposed command name to a source script within the package dir.
type Bin struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

// Manifest is the parsed contents of spm.toml.
type Manifest struct {
	Name        string `toml:"name"`
	Version     string `toml:"version"`
	Description string `toml:"description"`
	Bin         []Bin  `toml:"bin"`
}

// Load reads and validates spm.toml from the given package directory.
func Load(dir string) (*Manifest, error) {
	path := filepath.Join(dir, FileName)
	var m Manifest
	if _, err := toml.DecodeFile(path, &m); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := m.Validate(dir); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks the manifest is well-formed. dir may be empty to skip
// checking that each bin source file exists on disk.
func (m *Manifest) Validate(dir string) error {
	if !nameRe.MatchString(m.Name) {
		return fmt.Errorf("invalid package name %q: must match [a-z0-9-] and not start/end with '-'", m.Name)
	}
	if _, err := semver.StrictNewVersion(m.Version); err != nil {
		return fmt.Errorf("invalid version %q: must be strict semver (e.g. 1.4.2): %w", m.Version, err)
	}
	if len(m.Bin) == 0 {
		return fmt.Errorf("manifest must declare at least one [[bin]]")
	}
	seen := map[string]bool{}
	for _, b := range m.Bin {
		if b.Name == "" || b.Path == "" {
			return fmt.Errorf("each [[bin]] needs both name and path")
		}
		if !nameRe.MatchString(b.Name) {
			return fmt.Errorf("invalid bin name %q: must match [a-z0-9-]", b.Name)
		}
		if seen[b.Name] {
			return fmt.Errorf("duplicate bin name %q", b.Name)
		}
		seen[b.Name] = true
		if filepath.IsAbs(b.Path) {
			return fmt.Errorf("bin path %q must be relative to the package dir", b.Path)
		}
		if dir != "" {
			full := filepath.Join(dir, b.Path)
			if _, err := os.Stat(full); err != nil {
				return fmt.Errorf("bin %q source not found: %s", b.Name, b.Path)
			}
		}
	}
	return nil
}

// Artifact returns the tarball filename for this package, e.g. deploy-tools-1.4.2.tar.gz.
func (m *Manifest) Artifact() string {
	return fmt.Sprintf("%s-%s.tar.gz", m.Name, m.Version)
}
