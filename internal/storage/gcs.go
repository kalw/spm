package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	gcs "cloud.google.com/go/storage"

	"github.com/kalw/spm/internal/config"
)

// gcsStore backs native GCS access using Application Default Credentials.
type gcsStore struct {
	client *gcs.Client
	bucket string
}

func newGCS(ctx context.Context, r config.Remote) (Storage, error) {
	client, err := gcs.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcs client (need ADC / GOOGLE_APPLICATION_CREDENTIALS): %w", err)
	}
	return &gcsStore{client: client, bucket: r.Bucket}, nil
}

func (g *gcsStore) Put(ctx context.Context, key string, data []byte, contentType string) error {
	w := g.client.Bucket(g.bucket).Object(key).NewWriter(ctx)
	if contentType != "" {
		w.ContentType = contentType
	}
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return fmt.Errorf("gcs put %s: %w", key, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("gcs put %s: %w", key, err)
	}
	return nil
}

func (g *gcsStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	rc, err := g.client.Bucket(g.bucket).Object(key).NewReader(ctx)
	if err != nil {
		if errors.Is(err, gcs.ErrObjectNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("gcs get %s: %w", key, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func (g *gcsStore) Exists(ctx context.Context, key string) (bool, error) {
	_, err := g.client.Bucket(g.bucket).Object(key).Attrs(ctx)
	if err != nil {
		if errors.Is(err, gcs.ErrObjectNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("gcs attrs %s: %w", key, err)
	}
	return true, nil
}
