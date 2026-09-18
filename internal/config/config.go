// Package config loads named storage remotes from ~/.config/spm/config.toml.
//
// Secrets are never stored in the config file; they are read from the
// environment or the cloud SDK credential chains at use time.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Remote types.
const (
	TypeS3   = "s3"
	TypeGCS  = "gcs"
	TypeHTTP = "http"
)

// GCS access modes and HTTP publish modes.
const (
	GCSInterop = "interop"
	GCSNative  = "native"

	HTTPPublishWebDAV = "webdav"
	HTTPPublishLocal  = "local"

	GCSInteropEndpoint = "https://storage.googleapis.com"
)

// Remote describes one storage target. Not every field applies to every type.
type Remote struct {
	Type   string `toml:"type"`
	Bucket string `toml:"bucket"`
	Prefix string `toml:"prefix"`

	// s3 / gcs-interop
	Region   string `toml:"region"`
	Endpoint string `toml:"endpoint"`

	// gcs
	Access        string `toml:"access"`
	PublicBaseURL string `toml:"public_base_url"`

	// http
	Publish         string `toml:"publish"`
	PublishURL      string `toml:"publish_url"`
	PublishDir      string `toml:"publish_dir"`
	DownloadBaseURL string `toml:"download_base_url"`

	// name is filled in by Load for error messages.
	name string
}

// Config is the top-level config file.
type Config struct {
	Remotes map[string]Remote `toml:"remotes"`
}

// Path returns the config file path, honoring $SPM_CONFIG then $XDG_CONFIG_HOME.
func Path() string {
	if p := os.Getenv("SPM_CONFIG"); p != "" {
		return p
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "spm", "config.toml")
}

// Load reads the config file. A missing file is not an error (empty config).
func Load() (*Config, error) {
	c := &Config{Remotes: map[string]Remote{}}
	p := Path()
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return c, nil
	}
	if _, err := toml.DecodeFile(p, c); err != nil {
		return nil, fmt.Errorf("reading %s: %w", p, err)
	}
	for name, r := range c.Remotes {
		r.name = name
		c.Remotes[name] = r
	}
	return c, nil
}

// Remote resolves and validates a remote by name.
func (c *Config) Remote(name string) (Remote, error) {
	r, ok := c.Remotes[name]
	if !ok {
		return Remote{}, fmt.Errorf("remote %q not found in %s", name, Path())
	}
	r.name = name
	if err := r.validate(); err != nil {
		return Remote{}, err
	}
	return r, nil
}

func (r Remote) validate() error {
	switch r.Type {
	case TypeS3:
		if r.Bucket == "" {
			return fmt.Errorf("remote %q: s3 requires bucket", r.name)
		}
	case TypeGCS:
		if r.Bucket == "" {
			return fmt.Errorf("remote %q: gcs requires bucket", r.name)
		}
		if r.Access != GCSInterop && r.Access != GCSNative {
			return fmt.Errorf("remote %q: gcs access must be %q or %q", r.name, GCSInterop, GCSNative)
		}
	case TypeHTTP:
		if r.DownloadBaseURL == "" {
			return fmt.Errorf("remote %q: http requires download_base_url", r.name)
		}
		switch r.Publish {
		case HTTPPublishWebDAV:
			if r.PublishURL == "" {
				return fmt.Errorf("remote %q: http webdav requires publish_url", r.name)
			}
		case HTTPPublishLocal:
			if r.PublishDir == "" {
				return fmt.Errorf("remote %q: http local requires publish_dir", r.name)
			}
		default:
			return fmt.Errorf("remote %q: http publish must be %q or %q", r.name, HTTPPublishWebDAV, HTTPPublishLocal)
		}
	default:
		return fmt.Errorf("remote %q: unknown type %q", r.name, r.Type)
	}
	return nil
}

// Name returns the remote's configured name.
func (r Remote) Name() string { return r.name }

// Key builds the object key for a file within a package, applying the prefix.
// e.g. Key("deploy-tools", "versions.json") -> "spm/deploy-tools/versions.json".
func (r Remote) Key(pkg, file string) string {
	parts := []string{}
	if p := strings.Trim(r.Prefix, "/"); p != "" {
		parts = append(parts, p)
	}
	parts = append(parts, pkg, file)
	return strings.Join(parts, "/")
}
