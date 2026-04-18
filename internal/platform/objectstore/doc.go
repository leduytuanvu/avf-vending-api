// Package objectstore is an S3-compatible client (AWS S3, MinIO, etc.) for backend artifacts, OTA keys,
// diagnostic bundles, and similar blobs.
//
// When API_ARTIFACTS_ENABLED=true, cmd/api bootstrap constructs a Store from env and wires
// internal/app/artifacts (see internal/httpserver/artifacts_http.go). Key helpers include
// BackendArtifactObjectKey / OTAObjectKey / DiagnosticBundleKey.
//
// Typical environment (MinIO):
//
//	S3_BUCKET, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, S3_ENDPOINT (e.g. http://127.0.0.1:9000),
//	S3_USE_PATH_STYLE=true, AWS_REGION or S3_REGION (default us-east-1)
//
// Use New to construct a Store from Config, or ConfigFromEnv for process startup. PresignPut /
// PresignGet return raw URL + signed headers for thin HTTP handlers to redirect or proxy.
package objectstore
