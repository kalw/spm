// Package meta defines the per-version metadata sidecar published alongside a
// package artifact, so consumers can discover its dependencies without
// downloading the tarball.
package meta

import (
	"encoding/json"
	"fmt"

	"github.com/kalw/spm/internal/manifest"
)

// Meta is the content of <name>-<version>.spm.json.
type Meta struct {
	Name    string         `json:"name"`
	Version string         `json:"version"`
	Deps    []manifest.Dep `json:"deps,omitempty"`
}

// FileName returns the sidecar object name for this package version.
func FileName(name, version string) string {
	return fmt.Sprintf("%s-%s.spm.json", name, version)
}

// FileName returns the sidecar object name for this metadata.
func (m Meta) FileName() string { return FileName(m.Name, m.Version) }

// Encode renders the sidecar as pretty JSON with a trailing newline.
func Encode(m Meta) ([]byte, error) {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Parse decodes a sidecar body.
func Parse(body []byte) (Meta, error) {
	var m Meta
	if err := json.Unmarshal(body, &m); err != nil {
		return Meta{}, fmt.Errorf("parsing metadata sidecar: %w", err)
	}
	return m, nil
}
