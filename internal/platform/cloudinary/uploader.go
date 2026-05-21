package cloudinary

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/google/uuid"
)

// UploadInput is a server-side product image upload request.
type UploadInput struct {
	Reader      io.Reader
	Filename    string
	ContentType string
	SizeBytes   int64
	PublicID    string
	Purpose     string
	ActorID     uuid.UUID
	AppEnv      string
}

// UploadResult is the normalized provider response (no secrets).
type UploadResult struct {
	Provider     string
	PublicID     string
	AssetID      string
	DisplayURL   string
	SecureURL    string
	ThumbnailURL string
	Format       string
	Width        int
	Height       int
	Bytes        int64
	Version      int64
}

// Uploader uploads raster images to Cloudinary.
type Uploader struct {
	client      *cloudinary.Cloudinary
	folder      string
	thumbWidth  int
	thumbHeight int
}

// NewUploader builds a Cloudinary uploader. apiSecret must remain server-side only.
func NewUploader(cloudName, apiKey, apiSecret, folder string, thumbW, thumbH int) (*Uploader, error) {
	cloudName = strings.TrimSpace(cloudName)
	apiKey = strings.TrimSpace(apiKey)
	apiSecret = strings.TrimSpace(apiSecret)
	if cloudName == "" || apiKey == "" || apiSecret == "" {
		return nil, fmt.Errorf("cloudinary: cloud name, api key, and api secret are required")
	}
	cld, err := cloudinary.NewFromParams(cloudName, apiKey, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("cloudinary: init client: %w", err)
	}
	if strings.TrimSpace(folder) == "" {
		folder = "avf-vending/products"
	}
	if thumbW <= 0 {
		thumbW = 300
	}
	if thumbH <= 0 {
		thumbH = 300
	}
	return &Uploader{
		client:      cld,
		folder:      strings.Trim(folder, "/"),
		thumbWidth:  thumbW,
		thumbHeight: thumbH,
	}, nil
}

// UploadProductImage uploads bytes to Cloudinary and returns delivery URLs.
func (u *Uploader) UploadProductImage(ctx context.Context, in UploadInput) (UploadResult, error) {
	if u == nil || u.client == nil {
		return UploadResult{}, fmt.Errorf("cloudinary: uploader not configured")
	}
	publicID := strings.TrimSpace(in.PublicID)
	if publicID == "" {
		return UploadResult{}, fmt.Errorf("cloudinary: public id required")
	}
	tags := []string{"avf-vending", "product-image"}
	if env := strings.TrimSpace(in.AppEnv); env != "" {
		tags = append(tags, env)
	}
	ctxMeta := map[string]string{
		"purpose": strings.TrimSpace(in.Purpose),
	}
	if in.Purpose == "" {
		ctxMeta["purpose"] = "product_image"
	}
	if in.Filename != "" {
		ctxMeta["filename"] = in.Filename
	}
	if in.ActorID != uuid.Nil {
		ctxMeta["uploaded_by"] = in.ActorID.String()
	}
	params := uploader.UploadParams{
		PublicID:       publicID,
		Folder:         u.folder,
		ResourceType:   "image",
		Overwrite:      boolPtr(false),
		UseFilename:    boolPtr(false),
		UniqueFilename: boolPtr(false),
		Tags:           tags,
		Context:        ctxMeta,
		Invalidate:     boolPtr(false),
	}
	resp, err := u.client.Upload.Upload(ctx, in.Reader, params)
	if err != nil {
		return UploadResult{}, fmt.Errorf("cloudinary: upload failed: %w", err)
	}
	displayURL := strings.TrimSpace(resp.SecureURL)
	if displayURL == "" {
		displayURL = strings.TrimSpace(resp.URL)
	}
	thumbURL, err := u.thumbnailURL(strings.TrimSpace(resp.PublicID))
	if err != nil {
		thumbURL = displayURL
	}
	ver := int64(resp.Version)
	if ver <= 0 {
		ver = 1
	}
	return UploadResult{
		Provider:     "cloudinary",
		PublicID:     strings.TrimSpace(resp.PublicID),
		AssetID:      strings.TrimSpace(resp.AssetID),
		DisplayURL:   displayURL,
		SecureURL:    displayURL,
		ThumbnailURL: thumbURL,
		Format:       strings.TrimSpace(resp.Format),
		Width:        resp.Width,
		Height:       resp.Height,
		Bytes:        int64(resp.Bytes),
		Version:      ver,
	}, nil
}

func (u *Uploader) thumbnailURL(publicID string) (string, error) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return "", fmt.Errorf("cloudinary: empty public id")
	}
	img, err := u.client.Image(publicID)
	if err != nil {
		return "", err
	}
	img.Transformation = fmt.Sprintf("w_%d,h_%d,c_fill,q_auto,f_auto", u.thumbWidth, u.thumbHeight)
	return img.String()
}

func boolPtr(v bool) *bool {
	return &v
}
