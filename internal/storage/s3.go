package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"

	"github.com/kalw/spm/internal/config"
)

// s3Store backs both native S3 (and S3-compatible endpoints like MinIO) and
// GCS via its S3-interoperability endpoint.
type s3Store struct {
	client *s3.Client
	bucket string
}

func newS3(ctx context.Context, r config.Remote) (Storage, error) {
	region := r.Region
	endpoint := r.Endpoint
	var loadOpts []func(*awscfg.LoadOptions) error

	// GCS interop: fixed endpoint + HMAC keys from env.
	if r.Type == config.TypeGCS {
		endpoint = config.GCSInteropEndpoint
		if region == "" {
			region = "auto"
		}
		key, secret := os.Getenv("SPM_GCS_HMAC_KEY"), os.Getenv("SPM_GCS_HMAC_SECRET")
		if key == "" || secret == "" {
			return nil, fmt.Errorf("gcs interop requires SPM_GCS_HMAC_KEY and SPM_GCS_HMAC_SECRET")
		}
		loadOpts = append(loadOpts, awscfg.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(key, secret, "")))
	}
	if region != "" {
		loadOpts = append(loadOpts, awscfg.WithRegion(region))
	}

	cfg, err := awscfg.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = &endpoint
			o.UsePathStyle = true // safest for custom endpoints (MinIO, GCS)
		}
	})
	return &s3Store{client: client, bucket: r.Bucket}, nil
}

func (s *s3Store) Put(ctx context.Context, key string, data []byte, contentType string) error {
	in := &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
		Body:   bytes.NewReader(data),
	}
	if contentType != "" {
		in.ContentType = &contentType
	}
	_, err := s.client.PutObject(ctx, in)
	if err != nil {
		return fmt.Errorf("s3 put %s: %w", key, err)
	}
	return nil
}

func (s *s3Store) Get(ctx context.Context, key string) ([]byte, bool, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		if isNotFound(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("s3 get %s: %w", key, err)
	}
	defer out.Body.Close()
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func (s *s3Store) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("s3 head %s: %w", key, err)
	}
	return true, nil
}

func isNotFound(err error) bool {
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	// HeadObject returns a generic 404 API error rather than a typed NotFound.
	var api smithy.APIError
	if errors.As(err, &api) {
		switch api.ErrorCode() {
		case "NotFound", "NoSuchKey", "404":
			return true
		}
	}
	return false
}
