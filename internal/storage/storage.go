// Package storage writes and reads package artifacts to a private store.
//
// All payloads (tarballs, checksums, versions.json) are small and handled in
// memory, so the interface deals in []byte rather than streams.
package storage

import (
	"context"
	"fmt"

	"github.com/kalw/spm/internal/config"
)

// Storage is the write/read surface spm needs from a private store.
type Storage interface {
	// Put writes data at key, overwriting any existing object.
	Put(ctx context.Context, key string, data []byte, contentType string) error
	// Get returns the object at key. found is false (with nil error) when absent.
	Get(ctx context.Context, key string) (data []byte, found bool, err error)
	// Exists reports whether key is present.
	Exists(ctx context.Context, key string) (bool, error)
}

// New builds the Storage implementation for a remote.
func New(ctx context.Context, r config.Remote) (Storage, error) {
	switch r.Type {
	case config.TypeS3:
		return newS3(ctx, r)
	case config.TypeGCS:
		if r.Access == config.GCSInterop {
			return newS3(ctx, r) // GCS via its S3-interop endpoint
		}
		return newGCS(ctx, r)
	case config.TypeHTTP:
		return newHTTP(r)
	default:
		return nil, fmt.Errorf("unsupported remote type %q", r.Type)
	}
}
