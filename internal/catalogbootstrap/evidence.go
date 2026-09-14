package catalogbootstrap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// EvidenceWriter persists bootstrap audit files.
type EvidenceWriter struct {
	Dir string
}

func NewEvidenceWriter(dir string) (*EvidenceWriter, error) {
	if dir == "" {
		dir = ".catalog-bootstrap-evidence"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &EvidenceWriter{Dir: dir}, nil
}

func (w *EvidenceWriter) WriteJSON(name string, v any) error {
	if w == nil {
		return nil
	}
	path := filepath.Join(w.Dir, name)
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// Checkpoint tracks per-SKU import state for resume.
type Checkpoint struct {
	UpdatedAt string                    `json:"updated_at"`
	Items     map[string]CheckpointItem `json:"items"`
}

type CheckpointItem struct {
	State           string `json:"state"`
	MediaAssetID    string `json:"media_asset_id,omitempty"`
	ProductID       string `json:"product_id,omitempty"`
	CloudinaryPubID string `json:"cloudinary_public_id,omitempty"`
	Error           string `json:"error,omitempty"`
}

func (w *EvidenceWriter) LoadCheckpoint() (*Checkpoint, error) {
	path := filepath.Join(w.Dir, "import-state.json")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Checkpoint{Items: map[string]CheckpointItem{}}, nil
		}
		return nil, err
	}
	var cp Checkpoint
	if err := json.Unmarshal(b, &cp); err != nil {
		return nil, err
	}
	if cp.Items == nil {
		cp.Items = map[string]CheckpointItem{}
	}
	return &cp, nil
}

func (w *EvidenceWriter) SaveCheckpoint(cp *Checkpoint) error {
	if cp == nil {
		return nil
	}
	cp.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return w.WriteJSON("import-state.json", cp)
}

func RedactCloudinaryFingerprint(cloudName, folder, apiKey string) map[string]string {
	keyLast4 := ""
	if len(apiKey) >= 4 {
		keyLast4 = apiKey[len(apiKey)-4:]
	}
	return map[string]string{
		"cloud_name":   cloudName,
		"folder":       folder,
		"api_key_hint": fmt.Sprintf("****%s", keyLast4),
		"verified_at":  time.Now().UTC().Format(time.RFC3339),
	}
}
