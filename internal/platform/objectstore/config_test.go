package objectstore

import "testing"

func TestConfigFromEnv_ObjectStorageAliases(t *testing.T) {
	t.Setenv("OBJECT_STORAGE_BUCKET", "avf-vending-prod-assets")
	t.Setenv("OBJECT_STORAGE_ACCESS_KEY", "fixture")
	t.Setenv("OBJECT_STORAGE_SECRET_KEY", "fixture")
	t.Setenv("OBJECT_STORAGE_REGION", "ap-southeast-1")
	t.Setenv("OBJECT_STORAGE_ENDPOINT", "https://storage.example.com")

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Bucket != "avf-vending-prod-assets" {
		t.Fatalf("bucket: %q", cfg.Bucket)
	}
	if cfg.AccessKeyID != "fixture" || cfg.SecretAccessKey != "fixture" {
		t.Fatalf("unexpected credentials: %+v", cfg)
	}
	if cfg.Region != "ap-southeast-1" {
		t.Fatalf("region: %q", cfg.Region)
	}
	if cfg.Endpoint != "https://storage.example.com" {
		t.Fatalf("endpoint: %q", cfg.Endpoint)
	}
	if !cfg.UsePathStyle {
		t.Fatal("expected endpoint-based config to force path style")
	}
}

func TestConfigFromEnv_MissingBucketRejected(t *testing.T) {
	t.Setenv("OBJECT_STORAGE_ACCESS_KEY", "fixture")
	t.Setenv("OBJECT_STORAGE_SECRET_KEY", "fixture")

	_, err := ConfigFromEnv()
	if err == nil {
		t.Fatal("expected error")
	}
}
