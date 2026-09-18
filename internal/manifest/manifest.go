// Package manifest parses and validates the spm.toml package manifest.
package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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

// Dep is a runtime dependency provided by mise. It is emitted into the
// consumer's mise.toml by `spm add` and checked by the injected preflight.
type Dep struct {
	Mise    string `toml:"mise" json:"mise"`       // mise tool ref, e.g. "jq" or "npm:cowsay"
	Version string `toml:"version" json:"version"` // optional; defaults to "latest"
	Bin     string `toml:"bin" json:"bin"`         // optional; command checked at preflight
}

// Manifest is the parsed contents of spm.toml.
type Manifest struct {
	Name        string `toml:"name"`
	Version     string `toml:"version"`
	Description string `toml:"description"`
	Bin         []Bin  `toml:"bin"`
	Deps        []Dep  `toml:"deps"`
	// Preflight controls whether a dependency check is injected into shell
	// scripts at package time. Nil means enabled (the default when deps exist).
	Preflight *bool `toml:"preflight"`
}

// PreflightEnabled reports whether preflight injection should happen.
func (m *Manifest) PreflightEnabled() bool { return m.Preflight == nil || *m.Preflight }

// EffectiveVersion returns the dep's version, defaulting to "latest".
func (d Dep) EffectiveVersion() string {
	if d.Version == "" {
		return "latest"
	}
	return d.Version
}

// EffectiveBin returns the command the preflight checks for, deriving it from
// the mise ref when not set explicitly (strips the backend prefix and any path).
func (d Dep) EffectiveBin() string {
	if d.Bin != "" {
		return d.Bin
	}
	s := d.Mise
	if i := strings.LastIndex(s, ":"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// MiseRef returns the ref used in `mise use <ref>` hints, appending the version
// unless it is "latest".
func (d Dep) MiseRef() string {
	if v := d.EffectiveVersion(); v != "latest" {
		return d.Mise + "@" + v
	}
	return d.Mise
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
	depSeen := map[string]bool{}
	for _, d := range m.Deps {
		if strings.TrimSpace(d.Mise) == "" {
			return fmt.Errorf("each [[deps]] needs a mise ref")
		}
		if strings.ContainsAny(d.Mise, " \t") {
			return fmt.Errorf("invalid dep ref %q: must not contain whitespace", d.Mise)
		}
		if strings.Contains(d.Mise, "@") {
			return fmt.Errorf("dep ref %q must not include a version; use a separate version field", d.Mise)
		}
		if depSeen[d.Mise] {
			return fmt.Errorf("duplicate dependency %q", d.Mise)
		}
		depSeen[d.Mise] = true
	}
	return nil
}

// Artifact returns the tarball filename for this package, e.g. deploy-tools-1.4.2.tar.gz.
func (m *Manifest) Artifact() string {
	return fmt.Sprintf("%s-%s.tar.gz", m.Name, m.Version)
}
