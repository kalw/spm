//go:build integration

// MinIO-backed integration tests for the S3 storage adapter (which also serves
// the GCS S3-interop path). These are excluded from the default build; run them
// with `make integration`, or manually:
//
//	AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin \
//	SPM_IT_S3_ENDPOINT=http://127.0.0.1:9000 SPM_IT_S3_BUCKET=spm-it \
//	go test -tags integration -count=1 ./internal/storage/...
//
// The test is skipped unless SPM_IT_S3_ENDPOINT is set.
package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/kalw/spm/internal/config"
)

func itRemote(t *testing.T) config.Remote {
	t.Helper()
	endpoint := os.Getenv("SPM_IT_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("SPM_IT_S3_ENDPOINT not set; skipping MinIO integration test")
	}
	bucket := os.Getenv("SPM_IT_S3_BUCKET")
	if bucket == "" {
		bucket = "spm-it"
	}
	region := os.Getenv("SPM_IT_S3_REGION")
	if region == "" {
		region = "us-east-1"
	}
	return config.Remote{
		Type:     config.TypeS3,
		Bucket:   bucket,
		Prefix:   "it",
		Region:   region,
		Endpoint: endpoint,
	}
}

// ensureBucket creates the test bucket if it does not already exist.
func ensureBucket(ctx context.Context, t *testing.T, r config.Remote) {
	t.Helper()
	cfg, err := awscfg.LoadDefaultConfig(ctx, awscfg.WithRegion(r.Region))
	if err != nil {
		t.Fatalf("aws config: %v", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = &r.Endpoint
		o.UsePathStyle = true
	})
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &r.Bucket})
	if err != nil {
		var owned *types.BucketAlreadyOwnedByYou
		var exists *types.BucketAlreadyExists
		if !errors.As(err, &owned) && !errors.As(err, &exists) {
			t.Fatalf("create bucket %q: %v", r.Bucket, err)
		}
	}
}

func TestS3RoundTrip(t *testing.T) {
	ctx := context.Background()
	r := itRemote(t)
	ensureBucket(ctx, t, r)

	store, err := New(ctx, r)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	key := r.Key("roundtrip-pkg", "example-1.0.0.tar.gz")

	// Absent object.
	if ok, err := store.Exists(ctx, key); err != nil || ok {
		t.Fatalf("Exists(absent) = %v, %v; want false, nil", ok, err)
	}
	if _, found, err := store.Get(ctx, key); err != nil || found {
		t.Fatalf("Get(absent) = found %v, err %v; want false, nil", found, err)
	}

	// Write then read back.
	payload := []byte("hello minio\x00\x01binary")
	if err := store.Put(ctx, key, payload, "application/gzip"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if ok, err := store.Exists(ctx, key); err != nil || !ok {
		t.Fatalf("Exists(present) = %v, %v; want true, nil", ok, err)
	}
	got, found, err := store.Get(ctx, key)
	if err != nil || !found {
		t.Fatalf("Get(present) = found %v, err %v", found, err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("Get returned %q, want %q", got, payload)
	}

	// Overwrite is allowed at the storage layer (immutability is enforced above it).
	payload2 := []byte("second write")
	if err := store.Put(ctx, key, payload2, "application/gzip"); err != nil {
		t.Fatalf("Put overwrite: %v", err)
	}
	got2, _, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get after overwrite: %v", err)
	}
	if !bytes.Equal(got2, payload2) {
		t.Fatalf("after overwrite got %q, want %q", got2, payload2)
	}
}

func TestS3VersionsManifestKey(t *testing.T) {
	ctx := context.Background()
	r := itRemote(t)
	ensureBucket(ctx, t, r)

	store, err := New(ctx, r)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	key := r.Key("manifest-pkg", "versions.json")
	body := []byte(`["1.0.0","1.2.0"]` + "\n")
	if err := store.Put(ctx, key, body, "application/json"); err != nil {
		t.Fatalf("Put versions.json: %v", err)
	}
	got, found, err := store.Get(ctx, key)
	if err != nil || !found {
		t.Fatalf("Get versions.json: found %v, err %v", found, err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("versions.json mismatch: %q", got)
	}
}
