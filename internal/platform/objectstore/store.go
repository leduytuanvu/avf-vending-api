package objectstore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ObjectMeta is a shallow row for list/head operations (S3 Head/List metadata).
type ObjectMeta struct {
	Key          string
	Size         int64
	ContentType  string
	LastModified time.Time
	ETag         string
	UserMetadata map[string]string // e.g. sha256hex from PutObject metadata
}

// Store is the blob contract for OTA artifacts, diagnostic bundles, backend artifacts, and similar payloads.
type Store interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	// PutWithUserMetadata uploads with S3 object user metadata (x-amz-meta-* keys, unprefixed map keys per SDK).
	PutWithUserMetadata(ctx context.Context, key string, body io.Reader, size int64, contentType string, userMetadata map[string]string) error
	Get(ctx context.Context, key string) (body io.ReadCloser, contentType string, err error)
	PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (SignedHTTP, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (SignedHTTP, error)
	Head(ctx context.Context, key string) (ObjectMeta, error)
	Delete(ctx context.Context, key string) error
	ListPrefix(ctx context.Context, prefix string, maxKeys int32) ([]ObjectMeta, error)
}

// SignedHTTP is a presigned request snapshot suitable for redirects or client-side uploads.
type SignedHTTP struct {
	URL     string
	Method  string
	Headers http.Header
}

// S3Store implements Store against S3-compatible endpoints (including MinIO).
type S3Store struct {
	api    *s3.Client
	pre    *s3.PresignClient
	bucket string
}

var _ Store = (*S3Store)(nil)

// New constructs an S3Store from explicit configuration.
func New(ctx context.Context, cfg Config) (*S3Store, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("objectstore: bucket is required")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return nil, fmt.Errorf("objectstore: access key and secret are required")
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("objectstore: aws config: %w", err)
	}
	opts := []func(*s3.Options){
		func(o *s3.Options) {
			o.UsePathStyle = cfg.UsePathStyle
		},
	}
	if ep := strings.TrimSpace(cfg.Endpoint); ep != "" {
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(ep)
		})
	}
	cli := s3.NewFromConfig(awsCfg, opts...)
	return &S3Store{
		api:    cli,
		pre:    s3.NewPresignClient(cli),
		bucket: cfg.Bucket,
	}, nil
}

// PingBucket verifies credentials and bucket reachability (readiness). It does not create the bucket.
func (s *S3Store) PingBucket(ctx context.Context) error {
	if s == nil || s.api == nil {
		return fmt.Errorf("objectstore: nil store")
	}
	_, err := s.api.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err != nil {
		return fmt.Errorf("objectstore: head bucket %q: %w", s.bucket, err)
	}
	return nil
}

// NewFromEnv builds an S3Store using ConfigFromEnv.
func NewFromEnv(ctx context.Context) (*S3Store, error) {
	cfg, err := ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return New(ctx, cfg)
}

// Put uploads an object; size may be -1 when unknown (streaming without Content-Length).
func (s *S3Store) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	return s.PutWithUserMetadata(ctx, key, body, size, contentType, nil)
}

// PutWithUserMetadata uploads an object with optional S3 user metadata (server-side x-amz-meta-*).
func (s *S3Store) PutWithUserMetadata(ctx context.Context, key string, body io.Reader, size int64, contentType string, userMetadata map[string]string) error {
	if s == nil || s.api == nil {
		return fmt.Errorf("objectstore: nil store")
	}
	k := strings.TrimSpace(key)
	if k == "" {
		return fmt.Errorf("objectstore: key is required")
	}
	ct := strings.TrimSpace(contentType)
	if ct == "" {
		ct = "application/octet-stream"
	}
	in := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(k),
		Body:        body,
		ContentType: aws.String(ct),
	}
	if size >= 0 {
		in.ContentLength = aws.Int64(size)
	}
	if len(userMetadata) > 0 {
		in.Metadata = mapsClone(userMetadata)
	}
	_, err := s.api.PutObject(ctx, in)
	if err != nil {
		return fmt.Errorf("objectstore: put %s: %w", k, err)
	}
	return nil
}

// Get downloads an object. Caller must close the ReadCloser.
func (s *S3Store) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	if s == nil || s.api == nil {
		return nil, "", fmt.Errorf("objectstore: nil store")
	}
	k := strings.TrimSpace(key)
	if k == "" {
		return nil, "", fmt.Errorf("objectstore: key is required")
	}
	out, err := s.api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(k),
	})
	if err != nil {
		return nil, "", fmt.Errorf("objectstore: get %s: %w", k, err)
	}
	ct := ""
	if out.ContentType != nil {
		ct = aws.ToString(out.ContentType)
	}
	return out.Body, ct, nil
}

// Head returns object metadata without downloading the body.
func (s *S3Store) Head(ctx context.Context, key string) (ObjectMeta, error) {
	if s == nil || s.api == nil {
		return ObjectMeta{}, fmt.Errorf("objectstore: nil store")
	}
	k := strings.TrimSpace(key)
	if k == "" {
		return ObjectMeta{}, fmt.Errorf("objectstore: key is required")
	}
	out, err := s.api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(k),
	})
	if err != nil {
		return ObjectMeta{}, fmt.Errorf("objectstore: head %s: %w", k, err)
	}
	meta := mapObjectMeta(k, out)
	return meta, nil
}

// Delete removes an object. Missing keys are not treated as errors (idempotent delete).
func (s *S3Store) Delete(ctx context.Context, key string) error {
	if s == nil || s.api == nil {
		return fmt.Errorf("objectstore: nil store")
	}
	k := strings.TrimSpace(key)
	if k == "" {
		return fmt.Errorf("objectstore: key is required")
	}
	_, err := s.api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(k),
	})
	if err != nil {
		return fmt.Errorf("objectstore: delete %s: %w", k, err)
	}
	return nil
}

// ListPrefix lists object keys under a prefix (non-recursive flat keys; no delimiter walk).
func (s *S3Store) ListPrefix(ctx context.Context, prefix string, maxKeys int32) ([]ObjectMeta, error) {
	if s == nil || s.api == nil {
		return nil, fmt.Errorf("objectstore: nil store")
	}
	p := strings.TrimSpace(prefix)
	if p == "" {
		return nil, fmt.Errorf("objectstore: prefix is required")
	}
	if maxKeys <= 0 {
		maxKeys = 1000
	}
	if maxKeys > 1000 {
		maxKeys = 1000
	}
	out, err := s.api.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucket),
		Prefix:  aws.String(p),
		MaxKeys: aws.Int32(maxKeys),
	})
	if err != nil {
		return nil, fmt.Errorf("objectstore: list %q: %w", p, err)
	}
	var rows []ObjectMeta
	for _, ent := range out.Contents {
		key := aws.ToString(ent.Key)
		if key == "" {
			continue
		}
		rows = append(rows, ObjectMeta{
			Key:          key,
			Size:         aws.ToInt64(ent.Size),
			LastModified: aws.ToTime(ent.LastModified),
			ETag:         strings.Trim(aws.ToString(ent.ETag), `"`),
		})
	}
	return rows, nil
}

func mapObjectMeta(key string, out *s3.HeadObjectOutput) ObjectMeta {
	m := ObjectMeta{Key: key}
	if out == nil {
		return m
	}
	if out.ContentLength != nil {
		m.Size = *out.ContentLength
	}
	if out.ContentType != nil {
		m.ContentType = aws.ToString(out.ContentType)
	}
	if out.LastModified != nil {
		m.LastModified = *out.LastModified
	}
	if out.ETag != nil {
		m.ETag = strings.Trim(aws.ToString(out.ETag), `"`)
	}
	if len(out.Metadata) > 0 {
		m.UserMetadata = mapsClone(out.Metadata)
	}
	return m
}

func mapsClone(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// PresignPut issues a PUT URL for browser or edge uploads.
func (s *S3Store) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (SignedHTTP, error) {
	if s == nil || s.pre == nil {
		return SignedHTTP{}, fmt.Errorf("objectstore: nil store")
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	ct := strings.TrimSpace(contentType)
	if ct == "" {
		ct = "application/octet-stream"
	}
	out, err := s.pre.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(strings.TrimSpace(key)),
		ContentType: aws.String(ct),
	}, func(o *s3.PresignOptions) {
		o.Expires = ttl
	})
	if err != nil {
		return SignedHTTP{}, fmt.Errorf("objectstore: presign put: %w", err)
	}
	return signedFromPresignOutput(out), nil
}

// PresignGet issues a GET URL for short-lived downloads.
func (s *S3Store) PresignGet(ctx context.Context, key string, ttl time.Duration) (SignedHTTP, error) {
	if s == nil || s.pre == nil {
		return SignedHTTP{}, fmt.Errorf("objectstore: nil store")
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	out, err := s.pre.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(strings.TrimSpace(key)),
	}, func(o *s3.PresignOptions) {
		o.Expires = ttl
	})
	if err != nil {
		return SignedHTTP{}, fmt.Errorf("objectstore: presign get: %w", err)
	}
	return signedFromPresignOutput(out), nil
}

func signedFromPresignOutput(out *v4.PresignedHTTPRequest) SignedHTTP {
	if out == nil {
		return SignedHTTP{}
	}
	h := out.SignedHeader
	if h == nil {
		h = make(http.Header)
	}
	return SignedHTTP{URL: out.URL, Method: out.Method, Headers: h}
}
