package catalogadmin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/platform/objectstore"
	"github.com/google/uuid"
)

// ProductMediaDeps configures optional object storage for deterministic org/.../products/... media keys.
type ProductMediaDeps struct {
	Store objectstore.Store
	// MaxUploadBytes caps artifact reads (defaults when unset).
	MaxUploadBytes int64
	PresignTTL     time.Duration
}

func copyArtifactIntoProductDeterministicKeys(ctx context.Context, store objectstore.Store, organizationID, artifactID, productID uuid.UUID, maxBytes int64, presignTTL time.Duration) (displayURL, thumbURL, displayKey, normalizedMIME string, err error) {
	if store == nil {
		return "", "", "", "", errors.New("catalogadmin: nil object store")
	}
	srcKey := objectstore.BackendArtifactObjectKey(organizationID, artifactID)
	meta, err := store.Head(ctx, srcKey)
	if err != nil {
		return "", "", "", "", fmt.Errorf("catalogadmin: artifact head %w", mapHeadErr(err))
	}
	if meta.Size <= 0 || meta.Size > maxBytes {
		return "", "", "", "", fmt.Errorf("%w: artifact size invalid or exceeds max", ErrInvalidArgument)
	}
	ct := normalizeArtifactContentType(meta.ContentType)
	if err := ValidateProductImageMIME(ct); err != nil {
		return "", "", "", "", err
	}
	rc, _, err := store.Get(ctx, srcKey)
	if err != nil {
		return "", "", "", "", fmt.Errorf("catalogadmin: artifact get %w", mapHeadErr(err))
	}
	defer rc.Close()
	buf, err := io.ReadAll(io.LimitReader(rc, maxBytes+1))
	if err != nil {
		return "", "", "", "", err
	}
	if int64(len(buf)) != meta.Size {
		return "", "", "", "", fmt.Errorf("%w: artifact size mismatch", ErrInvalidArgument)
	}
	if int64(len(buf)) > maxBytes {
		return "", "", "", "", fmt.Errorf("%w: artifact exceeds max_upload_bytes", ErrInvalidArgument)
	}

	displayKey = objectstore.ProductMediaDisplayWebpKey(organizationID, productID)
	thumbKey := objectstore.ProductMediaThumbWebpKey(organizationID, productID)
	rdr := bytes.NewReader(buf)
	if err := store.Put(ctx, displayKey, rdr, int64(len(buf)), ct); err != nil {
		return "", "", "", "", err
	}
	if err := store.Put(ctx, thumbKey, bytes.NewReader(buf), int64(len(buf)), ct); err != nil {
		return "", "", "", "", err
	}

	if presignTTL <= 0 {
		presignTTL = 15 * time.Minute
	}
	ds, err := store.PresignGet(ctx, displayKey, presignTTL)
	if err != nil {
		return "", "", "", "", err
	}
	ts, err := store.PresignGet(ctx, thumbKey, presignTTL)
	if err != nil {
		return "", "", "", "", err
	}
	return ds.URL, ts.URL, displayKey, ct, nil
}

func mapHeadErr(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "nosuchkey") || strings.Contains(msg, "notfound") || strings.Contains(msg, "404") {
		return fmt.Errorf("%w: artifact object not found", ErrInvalidArgument)
	}
	return err
}

func bestEffortDeleteDeterministicProductMedia(ctx context.Context, store objectstore.Store, storageKey string, organizationID, productID uuid.UUID) {
	if store == nil {
		return
	}
	sk := strings.TrimSpace(storageKey)
	if sk == "" {
		return
	}
	org, pid, ok := objectstore.ParseProductMediaDisplayKey(sk)
	if !ok || org != organizationID || pid != productID {
		return
	}
	dk := objectstore.ProductMediaDisplayWebpKey(organizationID, productID)
	tk := objectstore.ProductMediaThumbWebpKey(organizationID, productID)
	_ = store.Delete(ctx, dk)
	_ = store.Delete(ctx, tk)
}
