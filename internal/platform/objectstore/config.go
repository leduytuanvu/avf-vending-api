package objectstore

import (
	"fmt"
	"os"
	"strings"
)

// Config holds static credentials and endpoint options for an S3-compatible API.
type Config struct {
	Region          string
	Bucket          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
	// PublicBaseURL optional HTTPS origin for documentation or gateways (e.g. CDN in front of the bucket).
	// Clients typically use PresignGet URLs; set via OBJECT_STORAGE_PUBLIC_BASE_URL when serving public manifests.
	PublicBaseURL string
}

// ConfigFromEnv loads Config from standard AWS/MinIO-style variables.
func ConfigFromEnv() (Config, error) {
	bucket := firstNonEmptyTrimmed(os.Getenv("OBJECT_STORAGE_BUCKET"), os.Getenv("S3_BUCKET"))
	if bucket == "" {
		return Config{}, fmt.Errorf("objectstore: OBJECT_STORAGE_BUCKET or S3_BUCKET is required")
	}
	ak := firstNonEmptyTrimmed(os.Getenv("OBJECT_STORAGE_ACCESS_KEY"), os.Getenv("AWS_ACCESS_KEY_ID"))
	sk := firstNonEmptyTrimmed(os.Getenv("OBJECT_STORAGE_SECRET_KEY"), os.Getenv("AWS_SECRET_ACCESS_KEY"))
	if ak == "" || sk == "" {
		return Config{}, fmt.Errorf("objectstore: OBJECT_STORAGE_ACCESS_KEY and OBJECT_STORAGE_SECRET_KEY (or AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY) are required")
	}
	region := firstNonEmptyTrimmed(os.Getenv("OBJECT_STORAGE_REGION"), os.Getenv("AWS_REGION"))
	if region == "" {
		region = firstNonEmptyTrimmed(os.Getenv("S3_REGION"))
	}
	if region == "" {
		region = "us-east-1"
	}
	endpoint := firstNonEmptyTrimmed(os.Getenv("OBJECT_STORAGE_ENDPOINT"), os.Getenv("S3_ENDPOINT"))
	pathStyle := strings.EqualFold(strings.TrimSpace(os.Getenv("S3_USE_PATH_STYLE")), "true")
	if endpoint != "" && !pathStyle {
		pathStyle = true
	}
	publicBase := firstNonEmptyTrimmed(os.Getenv("OBJECT_STORAGE_PUBLIC_BASE_URL"), os.Getenv("CDN_PUBLIC_BASE_URL"))
	return Config{
		Region:          region,
		Bucket:          bucket,
		Endpoint:        endpoint,
		AccessKeyID:     ak,
		SecretAccessKey: sk,
		UsePathStyle:    pathStyle,
		PublicBaseURL:   strings.TrimRight(strings.TrimSpace(publicBase), "/"),
	}, nil
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
