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
}

// ConfigFromEnv loads Config from standard AWS/MinIO-style variables.
func ConfigFromEnv() (Config, error) {
	bucket := strings.TrimSpace(os.Getenv("S3_BUCKET"))
	if bucket == "" {
		return Config{}, fmt.Errorf("objectstore: S3_BUCKET is required")
	}
	ak := strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID"))
	sk := strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY"))
	if ak == "" || sk == "" {
		return Config{}, fmt.Errorf("objectstore: AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are required")
	}
	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("S3_REGION"))
	}
	if region == "" {
		region = "us-east-1"
	}
	endpoint := strings.TrimSpace(os.Getenv("S3_ENDPOINT"))
	pathStyle := strings.EqualFold(strings.TrimSpace(os.Getenv("S3_USE_PATH_STYLE")), "true")
	if endpoint != "" && !pathStyle {
		pathStyle = true
	}
	return Config{
		Region:          region,
		Bucket:          bucket,
		Endpoint:        endpoint,
		AccessKeyID:     ak,
		SecretAccessKey: sk,
		UsePathStyle:    pathStyle,
	}, nil
}
